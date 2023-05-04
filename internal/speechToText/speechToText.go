package speechtotext

import (
	"context"
	"log"
	"os"

	"com.deablabs.teno-voice/internal/responder"
	"com.deablabs.teno-voice/pkg/deepgram"
	"github.com/Jeffail/gabs/v2"
	"github.com/gorilla/websocket"
)

var dg = deepgram.NewClient(os.Getenv("DEEPGRAM_API_KEY"))

// deepgram s2t sdk
func NewStream(ctx context.Context, onClose func(), responder *responder.Responder, userID string) (*websocket.Conn, error) {
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
			select {
			default:
				_, message, err := ws.ReadMessage()
				if err != nil {
					log.Println("Deepgram stream closed: ", err)
	
					// Check if the error is one of the handled timeout errors or payload error
					if websocket.IsCloseError(err, 1011, 1008) {
						onClose()
					}
	
					ctx.Done()
					return // Change this line
				}
	
				jsonParsed, jsonErr := gabs.ParseJSON(message)
				if jsonErr != nil {
					log.Println("Error parsing json: ", jsonErr)
					continue
				}

				// log.Printf("Full Deepgram response: %s", jsonParsed.String())

				transcription, ok := jsonParsed.Path("channel.alternatives.0.transcript").Data().(string)
				
				if ok && transcription != "" {
    				responder.NewTranscription(transcription)
    				log.Printf("User <%s>: %s", userID, transcription)
				}

			case <-ctx.Done():
				log.Println("Context cancelled")
			}
		}
	}()
	
	
	

	return ws, err
}
