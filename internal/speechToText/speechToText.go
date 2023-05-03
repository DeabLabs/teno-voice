package speechtotext

import (
	"context"
	"log"
	"os"

	"com.deablabs.teno-voice/internal/transcript"
	"com.deablabs.teno-voice/pkg/deepgram"
	"github.com/Jeffail/gabs/v2"
	"github.com/gorilla/websocket"
)

var dg = deepgram.NewClient(os.Getenv("DEEPGRAM_API_KEY"))

// deepgram s2t sdk
func NewStream(ctx context.Context, onClose func(), transcript transcript.Transcript, userID string) (*websocket.Conn, error) {
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
					log.Println("Error reading message: ", err)
					// TODO I think we actually want to signal over a channel that this stream is toast
					// then we want to setup a new stream and channel for the user
					// killing the context may also be helpful, I'm not sure yet
					ctx.Done()
					break
				}
	
				jsonParsed, jsonErr := gabs.ParseJSON(message)
				if jsonErr != nil {
					log.Println("Error parsing json: ", jsonErr)
					continue
				} 
				transcription := jsonParsed.Path("channel.alternatives.0.transcript").String()

				transcript.AddLine(transcription)

				log.Printf("User <%s>: %s", userID, transcription)
			case <-ctx.Done():
				log.Println("Context cancelled")
			}
		}
	}()
	

	return ws, err
}
