package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, addr string) (client *redis.Client, close func()) {
	rdb := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	close = func() {
		rdb.Close()
	}

	return rdb, close
}
