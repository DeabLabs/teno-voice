package responder

import (
	// "context"
	// "errors"
	"fmt"
	"io"
	"sync"

	// "strings"

	// "com.deablabs.teno-voice/internal/llm"
	"com.deablabs.teno-voice/internal/textToSpeech/azure"
	"com.deablabs.teno-voice/internal/transcript"
	"github.com/disgoorg/snowflake/v2"
	"mccoy.space/g/ogg"
)

type UserSpeakingState struct {
	UserID    snowflake.ID
	IsSpeaking bool
}

type Responder struct {
	transcript        *transcript.Transcript
	playAudioChannel  chan []byte
	speakingStateChan chan UserSpeakingState
	speakingUsers     map[snowflake.ID]bool
	speakingUsersMu   sync.Mutex
}


func NewResponder(playAudioChannel chan []byte) *Responder {
	responder := &Responder{
		playAudioChannel:  playAudioChannel,
		transcript:        transcript.NewTranscript(),
		speakingStateChan: make(chan UserSpeakingState),
		speakingUsers:     make(map[snowflake.ID]bool),
	}

	go responder.listenForSpeakingState()

	return responder
}

func (r *Responder) UpdateUserSpeakingState(userID snowflake.ID, isSpeaking bool) {
	r.speakingStateChan <- UserSpeakingState{
		UserID:    userID,
		IsSpeaking: isSpeaking,
	}
}

func (r *Responder) listenForSpeakingState() {
	for state := range r.speakingStateChan {
		r.speakingUsersMu.Lock()
		r.speakingUsers[state.UserID] = state.IsSpeaking
		r.speakingUsersMu.Unlock()

		// If a user is speaking, stop playing audio
	}
}

func (r *Responder) IsAnyUserSpeaking() bool {
	r.speakingUsersMu.Lock()
	defer r.speakingUsersMu.Unlock()

	for _, isSpeaking := range r.speakingUsers {
		if isSpeaking {
			return true
		}
	}

	return false
}


func (r *Responder) NewTranscription(line string) {
	r.transcript.AddLine(line)
	r.playTextInVoiceChannel(line)
	// r.Respond()
}

// Play a line of text as audio
func (r *Responder) playTextInVoiceChannel(line string) {
	fmt.Printf("Playing line: %s\n", line)
    audioReader, err := azure.TextToSpeech(line)
    if err != nil {
        fmt.Printf("Error generating speech: %v", err)
        return
    }
    defer audioReader.Close()

    // Create a new Ogg Decoder
    decoder := ogg.NewDecoder(audioReader)

    for {
        // Decode Ogg pages
        page, err := decoder.Decode()
        if err != nil {
            if err == io.EOF {
                break
            }
            fmt.Printf("Error decoding Ogg page: %s", err)
            return
        }

		// TODO try bundling packets instead of sending them individually

        // Extract raw Opus packets from each page
        for _, packet := range page.Packets {
            r.playAudioChannel <- packet
        }
    }
}

func (r *Responder) GetTranscript() *transcript.Transcript {
	return r.transcript
}

// func (r *Responder) Respond() {
// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// // 	transcript := `John: Hey everyone, how are you doing today?
// // Alice: Im doing great, thanks! How about you, John?
// // John: I'm good too, thanks for asking. Did you finish the project we were working on?
// // Alice: Yes, I managed to complete it yesterday. I'll send it to you later today.
// // John: Awesome, looking forward to checking it out.
// // Bob: Hey, sorry I'm late. What did I miss?
// // Alice: No worries, Bob. We were just talking about the project.
// // Bob: Oh, great. I'm excited to see the final result.
// // John: Me too. We should schedule a time to discuss it as a team.
// // Alice: Agreed. How about tomorrow at 3 pm?
// // Bob: Works for me.`

// 	// Get recent lines of the transcript
// 	lines := r.transcript.GetRecentLines(5)

// 	// Create the chat completion stream
// 	stream, err := llm.GetTranscriptResponseStream(lines, "openai", "gpt-3.5-turbo")
// 	if err != nil {
// 		fmt.Printf("Token stream error: %v\n", err)
// 		return
// 	}
// 	defer stream.Close()

// 	// Create a channel to emit sentences
// 	sentenceChan := make(chan string)

// 	// Start a goroutine to process sentences received from the channel
// 	// go processSentences(ctx, sentenceChan, audioStreamChan)

// 	fmt.Printf("Stream response: ")

// 	// Initialize a strings.Builder to build sentences from tokens
// 	var sentenceBuilder strings.Builder

// 	// Initialize a variable to store the previous token
// 	var previousToken string

// 	// Initialize a flag to check if the stream has ended
// 	streamEnded := false

// 	// Iterate over tokens received from the stream
// 	for !streamEnded {
// 		// Receive a token from the stream
// 		response, err := stream.Recv()

// 		// If the stream has ended, set the streamEnded flag to true
// 		if errors.Is(err, io.EOF) {
// 			fmt.Println("\nStream finished")
// 			streamEnded = true
// 		} else if err != nil {
// 			// If there is an error while receiving the token, close the channel and return
// 			fmt.Printf("\nStream error: %v\n", err)
// 			close(sentenceChan)
// 			return
// 		} else {
// 			// Extract the token from the response
// 			currentToken := response.Choices[0].Delta.Content

// 			// Print the token and add a newline
// 			fmt.Printf("%v\n", currentToken)

// 			// If there is a previous token, append it to the sentenceBuilder
// 			if previousToken != "" {
// 				sentenceBuilder.WriteString(previousToken)

// 				// If the previous token ends with a sentence-ending character and the current token starts with a whitespace, emit the sentence and reset the sentenceBuilder
// 				if isEndOfSentence(previousToken) && startsWithWhitespace(currentToken) {
// 					sentence := sentenceBuilder.String()
// 					sentenceChan <- sentence
// 					sentenceBuilder.Reset()
// 				}
// 			}

// 			// Set the previous token to be the current token for the next iteration
// 			previousToken = currentToken
// 		}
// 	}
// 	// Emit any remaining sentence, close the channel, and return
// 	if sentenceBuilder.Len() > 0 {
// 		sentenceChan <- sentenceBuilder.String()
// 	}

// 	close(sentenceChan)
// }

// // isEndOfSentence checks if a token ends with a sentence-ending character or a sentence-ending character followed by a quote
// func isEndOfSentence(token string) bool {
// 	endChars := []string{".", "!", "?", ";", ":", "-", "\n"}
// 	quoteChars := []string{"\"", "”", "“", "`", "'"}

// 	for _, endChar := range endChars {
// 		// Check if the token ends with any of the end characters
// 		if strings.HasSuffix(token, endChar) {
// 			return true
// 		}

// 		// Check if the token ends with an end character followed by a quote
// 		for _, quoteChar := range quoteChars {
// 			if strings.HasSuffix(token, endChar+quoteChar) {
// 				return true
// 			}
// 		}
// 	}

// 	return false
// }

// startsWithWhitespace checks if a token starts with a whitespace character
// func startsWithWhitespace(token string) bool {
// 	if len(token) == 0 {
// 		return false
// 	}
// 	firstChar := rune(token[0])
// 	return unicode.IsSpace(firstChar)
// }

// func processSentences(ctx context.Context, sentenceChan chan string, audioStreamChan chan []byte) {
// 	sentenceOrderChan := make(chan int)
// 	defer close(sentenceOrderChan)

// 	var orderCounter int
// 	var currentOrder int
// 	orderStreamMap := &sync.Map{}
// 	var wg sync.WaitGroup

// 	processStreamInOrder := func(order int, stream []byte) {
// 		orderStreamMap.Store(order, stream)

// 		for {
// 			v, ok := orderStreamMap.Load(currentOrder)
// 			if ok {
// 				audioStreamChan <- v.([]byte)
// 				orderStreamMap.Delete(currentOrder)
// 				currentOrder++
// 			} else {
// 				break
// 			}
// 		}
// 	}

// 	for sentence := range sentenceChan {
// 		fmt.Printf("\nReceived sentence: %v\n", sentence) // Print the received sentence

// 		order := orderCounter
// 		orderCounter++

// 		wg.Add(1)
// 		go func(sentence string, order int) {
// 			defer wg.Done()
// 			stream, err := azure.TextToSpeech(sentence)
// 			if err != nil {
// 				fmt.Printf("Error synthesizing speech: %v\n", err)
// 				return
// 			}
// 			sentenceOrderChan <- order
// 			select {
// 			case <-sentenceOrderChan:
// 				processStreamInOrder(order, stream)
// 			case <-ctx.Done():
// 				return
// 			}
// 		}(sentence, order)
// 	}

// 	wg.Wait()
// 	close(audioStreamChan)
// }