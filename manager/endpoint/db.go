package endpoint

import (
	"context"

	api "github.com/onpremless/go-client"
	"github.com/onpremless/opless/common/db"
	"github.com/onpremless/opless/manager/redis"
)

func GetEndpoint(ctx context.Context, id string) (*api.Endpoint, error) {
	return db.GetValue[api.Endpoint](ctx, "endpoint", id)(redis.Client)
}

func GetEndpoints(ctx context.Context) ([]*api.Endpoint, error) {
	return db.GetValues[api.Endpoint](ctx, "endpoint")(redis.Client)
}

func SetEndpoint(ctx context.Context, endpoint *api.Endpoint) error {
	return db.SetValue(ctx, "endpoint:"+endpoint.Id, endpoint)(redis.Client)
}

func FindEndpoint(ctx context.Context, predicate func(val *api.Endpoint) bool) (*api.Endpoint, error) {
	return db.FindValue(ctx, "endpoint", predicate)(redis.Client)
}
