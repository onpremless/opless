package lambda

import (
	"context"

	api "github.com/onpremless/go-client"
	"github.com/onpremless/opless/common/db"
	"github.com/onpremless/opless/manager/redis"
)

func GetLambda(ctx context.Context, id string) (*api.Lambda, error) {
	return db.GetValue[api.Lambda](ctx, "lambda", id)(redis.Client)
}

func GetRuntime(ctx context.Context, id string) (*api.Runtime, error) {
	return db.GetValue[api.Runtime](ctx, "runtime", id)(redis.Client)
}

func GetLambdas(ctx context.Context) ([]*api.Lambda, error) {
	return db.GetValues[api.Lambda](ctx, "lambda")(redis.Client)
}

func GetRuntimes(ctx context.Context) ([]*api.Runtime, error) {
	return db.GetValues[api.Runtime](ctx, "runtime")(redis.Client)
}

func SetLambda(ctx context.Context, lambda *api.Lambda) error {
	return db.SetValue(ctx, "lambda:"+lambda.Id, lambda)(redis.Client)
}

func SetRuntime(ctx context.Context, runtime *api.Runtime) error {
	return db.SetValue(ctx, "runtime:"+runtime.Id, runtime)(redis.Client)
}

func FindLambda(ctx context.Context, predicate func(val *api.Lambda) bool) (*api.Lambda, error) {
	return db.FindValue(ctx, "lambda", predicate)(redis.Client)
}
