package redis

import (
	"context"
	"fmt"

	"github.com/go-redis/redis/v8"

	"github.com/onpremless/opless/common/db"
	"github.com/onpremless/opless/common/util"
	"github.com/onpremless/opless/manager/logger"
)

var Client *db.Redis
var OPlessID = ""

func init() {
	redisEndpoint := util.GetStrVar("REDIS_ENDPOINT")
	var err error
	Client, err = db.NewRedis(redisEndpoint, logger.L)
	if err != nil {
		panic(fmt.Errorf("failed to connect to redis: %w", err))
	}

	res := Client.Client.Get(context.Background(), "opless-id")
	err = res.Err()
	if err == nil {
		OPlessID = res.Val()
		return
	}

	if err == redis.Nil {
		OPlessID = util.UUID()
		if err := Client.Client.Set(context.Background(), "opless-id", OPlessID, 0).Err(); err != nil {
			panic(err)
		}
	}
}
