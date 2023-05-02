package speechtotext

import (
	"context"
	"log"
	"os"

	"com.deablabs.teno-voice/pkg/deepgram"
	"github.com/Jeffail/gabs/v2"
	"github.com/gorilla/websocket"
)

var dg = deepgram.NewClient(os.Getenv("DEEPGRAM_API_KEY"))

// deepgram s2t sdk
func NewStream(ctx context.Context, userID string) (*websocket.Conn, error) {
	ws, _, err := dg.LiveTranscription(deepgram.LiveTranscriptionOptions{
		Punctuate:   true,
		Encoding:    "opus",
		Sample_rate: 48000,
		Channels:    2,
	})

	if err != nil {
		log.Println(err)
		return nil, err
	}

	go func() {
		for {
			_, message, err := ws.ReadMessage()
			if err != nil {
				log.Println("Error reading message: ", err)
				break
			}

			jsonParsed, jsonErr := gabs.ParseJSON(message)
			if jsonErr != nil {
				log.Println("Error parsing json: ", jsonErr)
				continue
			}
			log.Printf("User <%s>: %s", userID, jsonParsed.Path("channel.alternatives.0.transcript").String())
		}
	}()

	return ws, err
}
