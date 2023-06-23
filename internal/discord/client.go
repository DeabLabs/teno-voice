package discord

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/disgo/gateway"
)

func NewClient(ctx context.Context, token string) (bot.Client, func(), error) {
	// Create wait group to wait for the client to be ready
	wg := &sync.WaitGroup{}
	wg.Add(1)

	client, err := disgo.New(token,
		bot.WithGatewayConfigOpts(gateway.WithIntents(gateway.IntentGuilds, gateway.IntentGuildVoiceStates, gateway.IntentGuildMessages)),
		bot.WithEventListenerFunc(func(e *events.Ready) {
			wg.Done()
		}),
	)

	if err != nil {
		return nil, nil, fmt.Errorf("error creating client: %s", err)
	}

	// open the gateway connection to discord
	if err = client.OpenGateway(ctx); err != nil {
		return nil, nil, fmt.Errorf("error connecting to gateway: %s", err)
	}

	// wait for the client to be ready
	wg.Wait()

	// close the client when the program exits
	c := func() {
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()
		client.Close(ctx)
	}

	return client, c, nil
}
