package logger

import (
	"go.uber.org/zap"
)

var L *zap.Logger

func init() {
	var err error

	L, err = zap.NewProduction()

	if err != nil {
		panic(err)
	}
}
