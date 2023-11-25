package redis

import (
	"fmt"

	"github.com/onpremless/opless/common/db"
	"github.com/onpremless/opless/common/util"
	"github.com/onpremless/opless/router/logger"
)

var Client *db.Redis

func init() {
	redisEndpoint := util.GetStrVar("REDIS_ENDPOINT")
	var err error
	Client, err = db.NewRedis(redisEndpoint, logger.L)
	if err != nil {
		panic(fmt.Errorf("failed to connect to redis: %w", err))
	}
}
