package service

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/onpremless/opless/common/data"
	"github.com/onpremless/opless/router/logger"
	"go.uber.org/zap"
)

type Router struct {
	tree *data.ConcurrentPrefixTree[string]
}

func NewRouter() *Router {
	return &Router{
		tree: data.CreatePrefixTree[string](),
	}
}

func (r Router) Add(route string, lambda string) {
	logger.L.Info(
		"Adding new route",
		zap.String("route", route),
		zap.String("lambda", lambda),
	)
	r.tree.Add(route, &lambda)
}

func (r Router) Remove(route string) {
	r.tree.Remove(route)
}

func (r Router) Get(route string) (string, error) {
	logger.L.Info(
		"Getting route",
		zap.String("route", route),
	)

	lambda, match := r.tree.GetLastPayload(route)

	if lambda == nil {
		return "", errors.New("route is not found")
	}

	prefix := fmt.Sprintf("http://%s:3000", *lambda)

	if len(match) == len(route) {
		return prefix + "/", nil
	}

	rest := route[len(match):]
	if rest[0] != '/' {
		prefix += "/"
	}

	urlStr := prefix + rest
	if _, err := url.Parse(urlStr); err != nil {
		return "", err
	}

	return urlStr, nil
}
