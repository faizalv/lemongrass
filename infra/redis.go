package infra

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func NewRedis(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: addr,
	})
}

func PingRedis(ctx context.Context, rds *redis.Client) error {
	return rds.Ping(ctx).Err()
}
