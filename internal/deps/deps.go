package deps

import (
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	RedisClient *redis.Client
	Validate    *validator.Validate
}
