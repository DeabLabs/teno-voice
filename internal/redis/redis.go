package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, addr string) (client *redis.Client, close func()) {
	opt, err := redis.ParseURL(addr)
	if err != nil {
		panic(err)
	}

	rdb := redis.NewClient(opt)

	close = func() {
		rdb.Close()
	}

	return rdb, close
}
