package service

import (
	"context"

	api "github.com/onpremless/go-client"
	"github.com/onpremless/opless/common/db"
	"github.com/onpremless/opless/router/redis"
)

func GetEndpoints(ctx context.Context) ([]*api.Endpoint, error) {
	return db.GetValues[api.Endpoint](ctx, "endpoint")(redis.Client)
}

type NotificationHandler interface {
	HandleDel(key string)
	HandleSet(value *api.Endpoint)
}

func SubEndpointChanges(ctx context.Context, handler NotificationHandler) {
	notificationsC := db.Subscribe[api.Endpoint](ctx, "endpoint")(redis.Client)

	go func() {
		for notification := range notificationsC {
			switch n := notification.(type) {
			case *db.SetNotification[api.Endpoint]:
				handler.HandleSet(n.Value)
			case *db.DelNotification:
				handler.HandleDel(n.Key)
			}
		}
	}()
}
