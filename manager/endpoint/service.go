package endpoint

import (
	"context"
	"fmt"
	"time"

	api "github.com/onpremless/go-client"
	"github.com/onpremless/opless/common/util"
	"github.com/onpremless/opless/manager/lambda"
)

type EndpointService interface {
	List(ctx context.Context) ([]*api.Endpoint, error)
	Get(ctx context.Context, id string) (*api.Endpoint, error)
	Create(ctx context.Context, req *api.CreateEndpoint) (*api.Endpoint, error)
}

type endpointService struct {
	lambdaSvc lambda.LambdaService
}

func CreateEndpointService(lambdaSvc lambda.LambdaService) EndpointService {
	return &endpointService{
		lambdaSvc: lambdaSvc,
	}
}

func (s endpointService) List(ctx context.Context) ([]*api.Endpoint, error) {
	return GetEndpoints(ctx)
}

func (s endpointService) Get(ctx context.Context, id string) (*api.Endpoint, error) {
	return GetEndpoint(ctx, id)
}

func (s endpointService) Create(ctx context.Context, req *api.CreateEndpoint) (*api.Endpoint, error) {
	lambda, err := lambda.GetLambda(ctx, req.Lambda)
	if err != nil {
		return nil, err
	}

	if lambda == nil {
		return nil, fmt.Errorf("lambda is not found: %s", req.Lambda)
	}

	if lambda.LambdaType != "ENDPOINT" {
		return nil, fmt.Errorf("lamda is not an endpoint")
	}

	existingEndpoint, err := FindEndpoint(ctx, func(val *api.Endpoint) bool {
		return val.Path == req.Path
	})
	if err != nil {
		return nil, err
	}

	if existingEndpoint != nil {
		return nil, fmt.Errorf("endpoint already exists: %s", existingEndpoint.Id)
	}

	now := time.Now().UnixMilli()
	endpoint := &api.Endpoint{
		Id:        util.UUID(),
		Name:      req.Name,
		CreatedAt: now,
		UpdatedAt: now,
		Path:      req.Path,
		Lambda:    req.Lambda,
	}

	if err := SetEndpoint(ctx, endpoint); err != nil {
		return nil, err
	}

	return endpoint, nil
}
