package deps

import (
	"os"

	"github.com/disgoorg/disgo/bot"
)

type Deps struct {
	DiscordClient *bot.Client
	Signal        chan os.Signal
}
