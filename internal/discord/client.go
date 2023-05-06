package discord

import (
	"context"
	"fmt"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/gateway"
)

func NewClient(ctx context.Context, token string) (bot.Client, func(), error) {
	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildVoiceStates, gateway.IntentGuildMessages)),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("error creating client: %s", err)
	}

	// close the client when the program exits
	c := func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		client.Close(ctx)
	}

	// open the gateway connection to discord
	if err = client.OpenGateway(context.TODO()); err != nil {
		return nil, nil, fmt.Errorf("error connecting to gateway: %s", err)
	}

	return client, c, nil
}
