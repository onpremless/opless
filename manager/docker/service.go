package docker

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	api "github.com/onpremless/go-client"
	cutil "github.com/onpremless/opless/common/util"
	"github.com/onpremless/opless/manager/logger"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type service struct {
	client          *client.Client
	id              string
	internalNetwork string
}

type DockerService interface {
	Create(ctx context.Context, lambda *api.Lambda, tar io.Reader) (string, error)
	CreateContainer(ctx context.Context, lambda *api.Lambda) (string, error)
	Start(ctx context.Context, lambda *api.Lambda) error
	Stop(ctx context.Context, lambda *api.Lambda) error
	ListContainers(ctx context.Context) ([]types.Container, error)
	Inspect(ctx context.Context, id string) (types.ContainerJSON, error)
	Remove(ctx context.Context, lambda *api.Lambda) error
}

func NewDockerService(id string) (DockerService, error) {
	client, err := client.NewClientWithOpts(client.FromEnv)

	if err != nil {
		return nil, err
	}

	return &service{client: client, id: id, internalNetwork: cutil.GetStrVar("INTERNAL_NETWORK")}, nil
}

func (s service) ListContainers(ctx context.Context) ([]types.Container, error) {
	return s.client.ContainerList(ctx, types.ContainerListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{Key: "label", Value: "doless=" + s.id}),
	})
}

func (s service) Inspect(ctx context.Context, id string) (types.ContainerJSON, error) {
	return s.client.ContainerInspect(ctx, id)
}

func (s service) Create(ctx context.Context, lambda *api.Lambda, tar io.Reader) (string, error) {
	if lambda.Docker.Container == nil || lambda.Docker.Image == nil {
		return "", fmt.Errorf("lambda model is not complete")
	}

	images, err := s.client.ImageList(
		ctx,
		types.ImageListOptions{
			Filters: filters.NewArgs(filters.KeyValuePair{Key: "label", Value: "doless=" + s.id}),
		})
	if err != nil {
		return "", err
	}

	_, exists := lo.Find(images, func(image types.ImageSummary) bool {
		return len(image.RepoTags) > 0 && strings.Split(image.RepoTags[0], ":")[0] == *lambda.Docker.Image
	})
	if exists {
		return "", fmt.Errorf("image already exists: %s", *lambda.Docker.Image)
	}

	out, err := s.client.ImageBuild(ctx, tar, types.ImageBuildOptions{
		Tags:   []string{*lambda.Docker.Image},
		Labels: map[string]string{"doless": s.id},
	})
	if err != nil {
		return "", err
	}

	defer out.Body.Close()

	errorMsg := ""
	scanner := bufio.NewScanner(out.Body)
	for scanner.Scan() {
		e := struct {
			Err *string `json:"error"`
		}{}

		if err := json.Unmarshal([]byte(scanner.Text()), &e); err != nil {
			return "", err
		}

		if e.Err != nil {
			errorMsg += *e.Err
		}
	}

	if errorMsg != "" {
		return "", fmt.Errorf(errorMsg)
	}

	return s.CreateContainer(ctx, lambda)
}

func (s service) CreateContainer(ctx context.Context, lambda *api.Lambda) (string, error) {
	creator := &ContainerCreator{
		client: s.client,
		lambda: lambda,
	}

	err := creator.createContainer(ctx, &container.Config{
		Image:  *lambda.Docker.Image,
		Labels: map[string]string{"doless": s.id},
	})
	if err != nil {
		creator.rollback()
		return "", err
	}

	err = creator.setupNetwork(ctx, types.NetworkListOptions{
		Filters: filters.NewArgs(filters.KeyValuePair{
			Key:   "name",
			Value: s.internalNetwork,
		}),
	})

	if err != nil {
		creator.rollback()
		return "", err
	}

	return creator.container.ID, nil
}

func (s service) Start(ctx context.Context, lambda *api.Lambda) error {
	if lambda.Docker.ContainerId == nil {
		return fmt.Errorf("lambda model is not complete")
	}

	info, err := s.client.ContainerInspect(ctx, *lambda.Docker.ContainerId)
	if err != nil {
		return err
	}

	if info.State.Running || info.State.Restarting {
		return nil
	}

	if err := s.client.ContainerStart(ctx, *lambda.Docker.ContainerId, types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

func (s service) Stop(ctx context.Context, lambda *api.Lambda) error {
	if lambda.Docker.ContainerId == nil {
		return fmt.Errorf("lambda model is not complete")
	}

	info, err := s.client.ContainerInspect(ctx, *lambda.Docker.ContainerId)
	if err != nil {
		return err
	}

	if !info.State.Running && !info.State.Restarting {
		return nil
	}

	if err := s.client.ContainerStop(ctx, *lambda.Docker.ContainerId, container.StopOptions{}); err != nil {
		return err
	}

	return nil
}

func (s service) Remove(ctx context.Context, lambda *api.Lambda) error {
	if lambda.Docker.Container == nil || lambda.Docker.Image == nil {
		return fmt.Errorf("lambda model is not complete")
	}

	if err := s.Stop(ctx, lambda); err != nil {
		return err
	}

	if err := s.client.ContainerRemove(ctx, *lambda.Docker.ContainerId, types.ContainerRemoveOptions{}); err != nil {
		return err
	}

	if _, err := s.client.ImageRemove(ctx, *lambda.Docker.Image, types.ImageRemoveOptions{}); err != nil {
		return err
	}

	return nil
}

type ContainerCreator struct {
	client    *client.Client
	lambda    *api.Lambda
	container *container.CreateResponse
}

func (c *ContainerCreator) createContainer(ctx context.Context, conf *container.Config) error {
	container, err := c.client.ContainerCreate(ctx, conf, nil, nil, nil, *c.lambda.Docker.Container)
	if err != nil {
		return err
	}

	c.container = &container

	return nil
}

func (c *ContainerCreator) setupNetwork(ctx context.Context, opts types.NetworkListOptions) error {
	nets, err := c.client.NetworkList(ctx, opts)
	if err != nil {
		return err
	}

	if len(nets) != 1 {
		return errors.New("invalid networks length")
	}

	if err := c.client.NetworkConnect(ctx, nets[0].ID, c.container.ID, &network.EndpointSettings{Aliases: []string{c.lambda.Name}}); err != nil {
		return err
	}

	return nil
}

func (c *ContainerCreator) rollback() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if c.container != nil {
		err := c.client.ContainerRemove(ctx, c.container.ID, types.ContainerRemoveOptions{})
		if err != nil {
			logger.L.Error(
				"Failed to remove container",
				zap.Error(err),
				zap.String("id", c.container.ID),
			)
		} else {
			c.container = nil
		}
	}

	if _, err := c.client.ImageRemove(ctx, *c.lambda.Docker.Image, types.ImageRemoveOptions{}); err != nil {
		logger.L.Error(
			"Failed to remove image",
			zap.Error(err),
			zap.String("id", *c.lambda.Docker.Image),
		)
	}
}
