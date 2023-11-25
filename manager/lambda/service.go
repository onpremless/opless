package lambda

import (
	"context"
	"errors"
	"fmt"
	"time"

	api "github.com/onpremless/go-client"
	"github.com/onpremless/opless/common/data"
	cutil "github.com/onpremless/opless/common/util"
	"github.com/onpremless/opless/manager/docker"
	"github.com/onpremless/opless/manager/logger"
	"github.com/onpremless/opless/manager/redis"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type service struct {
	dockerSvc     docker.DockerService
	bootstrapping data.ConcurrentSet[string]
	starting      data.ConcurrentSet[string]
	lambdas       data.ConcurrentMap[string, api.Lambda]
	inspect       data.ConcurrentMap[string, func()]
}

type LambdaService interface {
	Init() error
	Stop(ctx context.Context)
	BootstrapRuntime(ctx context.Context, runtime *api.CreateRuntime) (*api.Runtime, error)
	BootstrapLambda(ctx context.Context, lambda *api.CreateLambda) (*api.Lambda, error)
	Start(ctx context.Context, id string) error
	Destroy(ctx context.Context, id string) error
}

func CreateLambdaService() (LambdaService, error) {
	dockerSvc, err := docker.NewDockerService(redis.DolessID)

	if err != nil {
		return nil, err
	}

	svc := &service{
		dockerSvc:     dockerSvc,
		bootstrapping: data.CreateConcurrentSet[string](),
		starting:      data.CreateConcurrentSet[string](),
		lambdas:       data.CreateConcurrentMap[string, api.Lambda](),
		inspect:       data.CreateConcurrentMap[string, func()](),
	}

	if err := svc.Init(); err != nil {
		return nil, err
	}

	return svc, nil
}

func (s service) Init() error {
	ctx := context.Background()
	lambdas, err := GetLambdas(ctx)

	if err != nil {
		return err
	}

	for _, lambda := range lambdas {
		s.lambdas.Set(lambda.Id, *lambda)
	}

	var lambdaInitErr error
	lo.ForEach(s.lambdas.Values(), func(lambda api.Lambda, _ int) {
		if lambda.Docker.ContainerId == nil {
			return
		}

		_, err := s.dockerSvc.Inspect(ctx, *lambda.Docker.ContainerId)

		// TODO: check error more precisely and handle correctly
		if err != nil {
			id, err := s.dockerSvc.CreateContainer(ctx, &lambda)
			if err != nil {
				id, lambdaInitErr = s.start(ctx, &lambda)
			}

			if id != "" {
				lambda.Docker.ContainerId = &id
				s.updateLambda(ctx, lambda)
			}
		} else if err := s.dockerSvc.Start(ctx, &lambda); err != nil {
			logger.L.Error(
				"failed to start lambda",
				zap.Error(err),
				zap.String("lambda", lambda.Id),
				zap.String("container_id", *lambda.Docker.ContainerId),
			)
		}
	})

	if lambdaInitErr != nil {
		return lambdaInitErr
	}

	s.lambdas.ForEach(func(_ string, lambda api.Lambda) {
		if lambda.Docker.ContainerId == nil {
			return
		}

		lctx, cancel := context.WithCancel(context.Background())
		s.inspect.Set(lambda.Id, cancel)
		go s.inspectRoutine(lctx, lambda)
	})

	return nil
}

func (s *service) Stop(ctx context.Context) {
	s.inspect.ForEach(func(_ string, stop func()) {
		stop()
	})

	s.lambdas.ForEach(func(_ string, lambda api.Lambda) {
		if lambda.Docker.ContainerId == nil {
			return
		}

		s.dockerSvc.Stop(ctx, &lambda)
	})
}

func (s *service) BootstrapRuntime(ctx context.Context, cRuntime *api.CreateRuntime) (*api.Runtime, error) {
	if succ := s.bootstrapping.AddUniq(cRuntime.Dockerfile); !succ {
		return nil, fmt.Errorf("lambda with '%s' archive is already in progress", cRuntime.Dockerfile)
	}
	defer s.bootstrapping.Remove(cRuntime.Dockerfile)

	id := cutil.UUID()

	if err := BootstrapRuntime(ctx, id, cRuntime); err != nil {
		return nil, err
	}

	createdAt := time.Now().UnixMilli()

	runtime := &api.Runtime{
		Id:        id,
		Name:      cRuntime.Name,
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	if err := SetRuntime(ctx, runtime); err != nil {
		return nil, err
	}

	return runtime, nil
}

func (s *service) BootstrapLambda(ctx context.Context, cLambda *api.CreateLambda) (*api.Lambda, error) {
	if succ := s.bootstrapping.AddUniq(cLambda.Archive); !succ {
		return nil, fmt.Errorf("lambda with '%s' archive is already being bootstrapped", cLambda.Archive)
	}
	defer s.bootstrapping.Remove(cLambda.Archive)

	existing, err := GetLambda(ctx, cLambda.Name)
	if err != nil {
		return nil, err
	}

	if existing != nil {
		return nil, errors.New("already exists")
	}

	if runtime, err := GetRuntime(ctx, cLambda.Runtime); err != nil {
		return nil, err
	} else if runtime == nil {
		return nil, errors.New("not found")
	}

	if err := BootstrapLambda(ctx, cLambda.Name, cLambda); err != nil {
		return nil, err
	}

	createdAt := time.Now().UnixMilli()

	lambda := api.Lambda{
		Id:         cLambda.Name,
		Name:       cLambda.Name,
		CreatedAt:  createdAt,
		UpdatedAt:  createdAt,
		Runtime:    cLambda.Runtime,
		LambdaType: cLambda.LambdaType,
	}

	if err := SetLambda(ctx, &lambda); err != nil {
		return nil, err
	}

	s.lambdas.Set(lambda.Id, lambda)

	return &lambda, nil
}

func (s service) start(ctx context.Context, lambda *api.Lambda) (string, error) {
	tar, err := TarLambda(ctx, lambda.Id, lambda.Runtime)
	if err != nil {
		return "", err
	}

	image := lambda.Name
	container := "doless-" + lambda.Name
	lambda.Docker.Image = &image
	lambda.Docker.Container = &container

	containerID, err := s.dockerSvc.Create(ctx, lambda, tar)
	if err != nil {
		return "", err
	}

	lambda.Docker.ContainerId = &containerID

	return containerID, s.dockerSvc.Start(ctx, lambda)
}

func (s service) Start(ctx context.Context, id string) error {
	if succ := s.starting.AddUniq(id); !succ {
		return fmt.Errorf("lambda '%s' is already being processed", id)
	}
	defer s.starting.Remove(id)

	lambda, err := GetLambda(ctx, id)
	if err != nil {
		return err
	}

	if lambda == nil {
		return errors.New("not found")
	}

	if _, err = s.start(ctx, lambda); err != nil {
		return err
	}

	if err := s.updateLambda(ctx, *lambda); err != nil {
		return err
	}

	lctx, cancel := context.WithCancel(context.Background())
	s.inspect.Set(lambda.Id, cancel)
	go s.inspectRoutine(lctx, *lambda)

	return nil
}

func (s service) Destroy(ctx context.Context, id string) error {
	if succ := s.starting.AddUniq(id); !succ {
		return fmt.Errorf("lambda '%s' is already being processed", id)
	}
	defer s.starting.Remove(id)

	lambda, err := GetLambda(ctx, id)
	if err != nil {
		return err
	}

	if lambda == nil {
		return errors.New("not found")
	}

	s.inspect.Get(id, func() {})()
	s.inspect.Delete(id)

	if err := s.dockerSvc.Remove(ctx, lambda); err != nil {
		return err
	}

	lambda.Docker = api.Docker{}

	if err := s.updateLambda(ctx, *lambda); err != nil {
		return err
	}

	return nil
}

func (s service) updateLambda(ctx context.Context, lambda api.Lambda) error {
	var updateErr error
	s.lambdas.Update(lambda.Id, func(prev api.Lambda) api.Lambda {
		if err := SetLambda(ctx, &lambda); err != nil {
			updateErr = err
			return prev
		}

		return lambda
	})

	return updateErr
}

func (s service) inspectRoutine(ctx context.Context, lambda api.Lambda) {
	id := *lambda.Docker.ContainerId
	for {
		container, err := s.dockerSvc.Inspect(ctx, id)
		actual, rErr := GetLambda(ctx, lambda.Id)
		if rErr != nil && actual == nil {
			rErr = errors.New("not found")
		}

		if rErr == nil {
			if err != nil {
				// TODO: Handle external delete case and stop
				logger.L.Error(
					"Failed to inspect container",
					zap.Error(err),
					zap.String("container_id", id),
				)
				lambda.Docker.Status = "error"
			} else {
				lambda.Docker.Status = container.State.Health.Status
			}

			if actual.Docker.Status != lambda.Docker.Status {
				if err := s.updateLambda(ctx, lambda); err != nil {
					logger.L.Error(
						"Failed to update lambda",
						zap.Error(err),
						zap.String("id", lambda.Id),
					)
				}
			}
		} else {
			logger.L.Error(
				"Failed to gather lambda",
				zap.Error(rErr),
				zap.String("id", lambda.Id),
			)
		}

		select {
		case <-time.After(10 * time.Second):
			continue
		case <-ctx.Done():
			return
		}
	}
}
