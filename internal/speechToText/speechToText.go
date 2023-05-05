package speechtotext

import (
	"context"
	"log"

	Config "com.deablabs.teno-voice/internal/config"
	"com.deablabs.teno-voice/internal/responder"
	"com.deablabs.teno-voice/pkg/deepgram"
	"github.com/Jeffail/gabs/v2"
	"github.com/disgoorg/snowflake/v2"
	"github.com/gorilla/websocket"
)

var dg = deepgram.NewClient(Config.Environment.DeepgramToken)

// deepgram s2t sdk
func NewStream(ctx context.Context, onClose func(), responder *responder.Responder, userID snowflake.ID) (*websocket.Conn, error) {
	ws, _, err := dg.LiveTranscription(deepgram.LiveTranscriptionOptions{
		Punctuate:       true,
		Encoding:        "opus",
		Sample_rate:     48000,
		Channels:        2,
		Interim_results: true,
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
					// log.Println("Deepgram stream closed: ", err)

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

				// If the transcription isn't empty, and the transcription isn't final, update the user's speaking state and send the transcription to the responder if it's final
				if ok && transcription != "" {
					if !jsonParsed.Path("is_final").Data().(bool) {
						responder.UpdateUserSpeakingState(userID, true)
					} else {
						responder.UpdateUserSpeakingState(userID, false)
						responder.NewTranscription(transcription)
						// log.Printf("User <%s>: %s", userID.String(), transcription)
					}
					// Print whether anyone is currently speaking
					//log.Printf("Speaking state: %t", responder.IsAnyUserSpeaking())
				}

			case <-ctx.Done():
				log.Println("Context cancelled")
			}
		}
	}()

	return ws, err
}
