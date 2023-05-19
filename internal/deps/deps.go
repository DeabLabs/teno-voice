package deps

import (
	"github.com/disgoorg/disgo/bot"
	"github.com/go-playground/validator/v10"
	"github.com/redis/go-redis/v9"
)

type Deps struct {
	DiscordClient *bot.Client
	RedisClient   *redis.Client
	Validate      *validator.Validate
}
