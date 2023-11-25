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
var DolessID = ""

func init() {
	redisEndpoint := util.GetStrVar("REDIS_ENDPOINT")
	var err error
	Client, err = db.NewRedis(redisEndpoint, logger.L)
	if err != nil {
		panic(fmt.Errorf("failed to connect to redis: %w", err))
	}

	res := Client.Client.Get(context.Background(), "doless-id")
	err = res.Err()
	if err == nil {
		DolessID = res.Val()
		return
	}

	if err == redis.Nil {
		DolessID = util.UUID()
		if err := Client.Client.Set(context.Background(), "doless-id", DolessID, 0).Err(); err != nil {
			panic(err)
		}
	}
}
