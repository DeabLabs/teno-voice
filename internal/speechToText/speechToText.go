package speechtotext

import (
	"context"
	"log"
	"strings"

	Config "com.deablabs.teno-voice/internal/config"
	"com.deablabs.teno-voice/internal/responder"
	"com.deablabs.teno-voice/internal/usage"
	"com.deablabs.teno-voice/pkg/deepgram"
	"github.com/Jeffail/gabs/v2"
	"github.com/gorilla/websocket"
)

var dg = deepgram.NewClient(Config.Environment.DeepgramToken)

type TranscriberConfig struct {
	Keywords     []string
	IgnoredUsers []string
}

type Transcriber struct {
	BotName         string
	Config          TranscriberConfig
	IgnoredUsersMap map[string]struct{}
	Responder       *responder.Responder
}

func NewTranscriber(botName string, config TranscriberConfig, responder *responder.Responder) *Transcriber {
	ignoredUsersMap := make(map[string]struct{})
	for _, ignoredUser := range config.IgnoredUsers {
		ignoredUsersMap[ignoredUser] = struct{}{}
	}
	return &Transcriber{
		BotName:         botName,
		Config:          config,
		IgnoredUsersMap: ignoredUsersMap,
		Responder:       responder,
	}
}

// deepgram s2t sdk
func (t *Transcriber) NewStream(ctx context.Context, onClose func(), username string, userId string) (*websocket.Conn, error) {
	// Split botname into words
	botNameWords := strings.Split(t.BotName, " ")

	ws, _, err := dg.LiveTranscription(deepgram.LiveTranscriptionOptions{
		Punctuate:       true,
		Encoding:        "opus",
		Sample_rate:     48000,
		Channels:        2,
		Interim_results: true,
		Search:          botNameWords,
		Keywords:        append(t.Config.Keywords, botNameWords...),
		Model:           "phonecall",
		Tier:            "nova",
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

				if ok {
					if !jsonParsed.Path("is_final").Data().(bool) {
						t.Responder.InterimTranscriptionReceived()
					} else {
						// Check if the bot name was spoken
						botNameConfidence := float64(0)
						searchResults := jsonParsed.Path("channel.search").Children()
						for _, searchResult := range searchResults {
							hits := searchResult.Path("hits").Children()
							for _, hit := range hits {
								confidence, _ := hit.Path("confidence").Data().(float64)
								if confidence > botNameConfidence {
									botNameConfidence = confidence
								}
							}
						}
						if transcription != "" {
							t.Responder.NewTranscription(transcription, botNameConfidence, username, userId)
						}

						usageEvent := usage.NewTranscriptionEvent("deepgram", "nova-streaming", jsonParsed.Path("duration").Data().(float64)/60.0)

						// Send event struct if its not empty
						if !usageEvent.IsEmpty() {
							usage.SendEventToDB(usageEvent)
						}
					}
				}

			case <-ctx.Done():
				log.Println("Context cancelled")
			}
		}
	}()

	return ws, err
}

func (t *Transcriber) IsIgnored(userId string) bool {
	_, ok := t.IgnoredUsersMap[userId]
	return ok
}
