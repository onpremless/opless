package service

import (
	"context"
	"net/http"

	api "github.com/onpremless/go-client"
	"github.com/onpremless/opless/common/data"
	"github.com/samber/lo"
)

type Service interface {
	RedirectURL(ctx context.Context, req *http.Request) (string, error)
	Stop()
}

type service struct {
	router    *Router
	endpoints data.ConcurrentMap[string, *api.Endpoint]
	stop      func()
}

func NewService(ctx context.Context) (Service, error) {
	s := &service{
		router:    NewRouter(),
		endpoints: data.CreateConcurrentMap[string, *api.Endpoint](),
	}

	if err := s.init(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *service) init(ctx context.Context) error {
	endpoints, err := GetEndpoints(ctx)
	if err != nil {
		return err
	}

	lo.ForEach(endpoints, func(endpoint *api.Endpoint, _ int) {
		s.endpoints.Set(endpoint.Id, endpoint)
		s.router.Add(endpoint.Path, endpoint.Lambda)
	})

	cancelCtx, stop := context.WithCancel(context.Background())
	s.stop = stop

	SubEndpointChanges(cancelCtx, s)

	return nil
}

func (s service) RedirectURL(ctx context.Context, req *http.Request) (string, error) {
	return s.router.Get(req.URL.String())
}

func (s service) Stop() {
	s.stop()
}

func (s service) HandleSet(endpoint *api.Endpoint) {
	prev := s.endpoints.Get(endpoint.Id, nil)
	if prev != nil {
		s.router.Remove(prev.Path)
	}

	s.endpoints.Set(endpoint.Id, endpoint)
	s.router.Add(endpoint.Path, endpoint.Lambda)
}

func (s service) HandleDel(key string) {
	endpoint := s.endpoints.Get(key, nil)
	if endpoint == nil {
		return
	}

	s.router.Remove(endpoint.Path)
}
