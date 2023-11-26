package db

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"go.uber.org/zap"
)

type Redis struct {
	Client *redis.Client
	L      *zap.Logger
}

func NewRedis(endpoint string, l *zap.Logger) (*Redis, error) {
	svc := &Redis{
		Client: redis.NewClient(&redis.Options{
			Addr: endpoint,
		}),
		L: l,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for i := 0; i < 5; i++ {
		if err := svc.Client.Ping(ctx).Err(); err == nil {
			return svc, nil
		}

		time.Sleep(time.Second)
	}

	return nil, errors.New("failed to connect to redis")
}

func scanValues[T any](ctx context.Context, prefix string, handler func(x *T) bool) func(r *Redis) error {
	return func(r *Redis) error {
		var cursor uint64
		traversed := map[string]bool{}

		for {
			var keys []string
			var err error

			keys, cursor, err = r.Client.Scan(ctx, cursor, prefix+":*", 0).Result()

			if err != nil {
				r.L.Error(
					"Failed to scan redis",
					zap.Error(err),
				)
				return err
			}

			for _, key := range keys {
				if traversed[key] {
					continue
				}

				traversed[key] = true
				val, err := getValueByKey[T](ctx, key)(r)
				if err != nil {
					continue
				}

				if !handler(val) {
					return nil
				}
			}

			if cursor == 0 {
				return nil
			}
		}
	}
}

func GetValues[T any](ctx context.Context, prefix string) func(r *Redis) ([]*T, error) {
	return func(r *Redis) ([]*T, error) {
		res := []*T{}

		err := scanValues(ctx, prefix, func(val *T) bool {
			res = append(res, val)
			return true
		})(r)

		if err != nil {
			return nil, err
		}

		return res, nil
	}
}

func getValueByKey[T any](ctx context.Context, key string) func(r *Redis) (*T, error) {
	return func(r *Redis) (*T, error) {
		rawVal, err := r.Client.Get(ctx, key).Result()
		if err != nil {
			if err == redis.Nil {
				return nil, nil
			}

			r.L.Error(
				"Failed to get value by key",
				zap.Error(err),
				zap.String("key", key),
			)
			return nil, err
		}

		var val T

		if err := json.Unmarshal([]byte(rawVal), &val); err != nil {
			r.L.Error(
				"Failed to parse redis value",
				zap.Error(err),
				zap.String("key", key),
				zap.String("value", rawVal),
			)
			return nil, err
		}

		return &val, err
	}
}

func GetValue[T any](ctx context.Context, prefix string, id string) func(r *Redis) (*T, error) {
	return getValueByKey[T](ctx, prefix+":"+id)
}

func SetValue(ctx context.Context, key string, val interface{}) func(r *Redis) error {
	return func(r *Redis) error {
		obj, err := json.Marshal(val)
		if err != nil {
			return err
		}

		status := r.Client.Set(ctx, key, string(obj), 0)
		if err := status.Err(); err != nil {
			return err
		}

		return nil
	}
}

func FindValue[T any](ctx context.Context, prefix string, predicate func(x *T) bool) func(r *Redis) (*T, error) {
	return func(r *Redis) (*T, error) {
		var res *T
		err := scanValues(ctx, prefix, func(val *T) bool {
			if predicate(val) {
				res = val
				return false
			}

			return true
		})(r)

		return res, err
	}
}

type SetNotification[T any] struct {
	Value *T
}

type DelNotification struct {
	Key string
}

func Subscribe[T any](ctx context.Context, prefix string) func(r *Redis) <-chan interface{} {
	return func(r *Redis) <-chan interface{} {
		topic := "__keyspace@0__:" + prefix + "*"
		r.L.Info("Subscribe to topic", zap.String("topic", topic))
		pubsub := r.Client.PSubscribe(ctx, topic)
		notificationsC := make(chan interface{})

		go func() {
			defer close(notificationsC)
			r.L.Info("Wait for notifications")

			for msg := range pubsub.Channel() {
				r.L.Info("New notification", zap.String("channel", msg.Channel))
				tSlice := strings.SplitN(msg.Channel, ":", 2)
				if len(tSlice) < 2 {
					r.L.Error(
						"Unexpected notification",
						zap.String("msg", msg.Channel),
					)
					continue
				}

				t := msg.Payload
				if t == "del" {
					notificationsC <- &DelNotification{msg.Payload}
					continue
				}

				val, err := getValueByKey[T](ctx, tSlice[1])(r)
				if err != nil {
					r.L.Error(
						"Failed to get redis value",
						zap.Error(err),
						zap.String("key", msg.Payload),
					)
					continue
				}

				notificationsC <- &SetNotification[T]{val}
			}
		}()

		return notificationsC
	}
}
