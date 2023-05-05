package responder

import (
	// "context"
	// "errors"

	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"unicode"

	// "strings"

	// "com.deablabs.teno-voice/internal/llm"
	"com.deablabs.teno-voice/internal/llm"
	texttospeech "com.deablabs.teno-voice/internal/textToSpeech"
	"com.deablabs.teno-voice/internal/transcript"
	"github.com/disgoorg/snowflake/v2"
)

type UserSpeakingState struct {
	UserID     snowflake.ID
	IsSpeaking bool
}

type audioStreamWithIndex struct {
	index       int
	opusPackets <-chan []byte
}

type Responder struct {
	transcript        *transcript.Transcript
	playAudioChannel  chan []byte
	audioStreamChan   chan audioStreamWithIndex
	speakingStateChan chan UserSpeakingState
	speakingUsers     map[snowflake.ID]bool
	speakingUsersMu   sync.Mutex
	sentenceChan      chan string
	ttsService        texttospeech.TextToSpeechService
}

func NewResponder(playAudioChannel chan []byte, ttsService texttospeech.TextToSpeechService) *Responder {
	responder := &Responder{
		playAudioChannel:  playAudioChannel,
		sentenceChan:      make(chan string, 100),
		audioStreamChan:   make(chan audioStreamWithIndex, 100),
		transcript:        transcript.NewTranscript(),
		speakingStateChan: make(chan UserSpeakingState),
		speakingUsers:     make(map[snowflake.ID]bool),
		ttsService:        ttsService,
	}

	go responder.listenForSpeakingState()
	// go responder.synthesizeSentences()
	// go responder.playSynthesizedSentences()

	return responder
}

func (r *Responder) UpdateUserSpeakingState(userID snowflake.ID, isSpeaking bool) {
	r.speakingStateChan <- UserSpeakingState{
		UserID:     userID,
		IsSpeaking: isSpeaking,
	}
}

// func (r *Responder) synthesizeSentences() {
// 	sentenceIndex := 0
// 	for sentence := range r.sentenceChan {
// 		log.Printf("Synthesizing sentence: %s\n", sentence)
// 		opusPackets, err := r.ttsService.Synthesize(sentence)
// 		if err != nil {
// 			fmt.Printf("Error generating speech: %v\n", err)
// 			continue
// 		}
// 		r.audioStreamChan <- audioStreamWithIndex{
// 			index:       sentenceIndex,
// 			opusPackets: opusPackets,
// 		}
// 		sentenceIndex++
// 	}
// }

// func (r *Responder) playSynthesizedSentences() {
// 	audioStreamMap := make(map[int]<-chan []byte)
// 	nextAudioIndex := 0

// 	for audioStreamWithIndex := range r.audioStreamChan {
// 		audioStreamMap[audioStreamWithIndex.index] = audioStreamWithIndex.opusPackets

// 		for {
// 			opusPackets, ok := audioStreamMap[nextAudioIndex]
// 			if !ok {
// 				break
// 			}

// 			for packet := range opusPackets {
// 				r.playAudioChannel <- packet
// 			}

// 			// Remove the played audio stream from the map and increment the nextAudioIndex
// 			delete(audioStreamMap, nextAudioIndex)
// 			nextAudioIndex++
// 		}
// 	}
// }

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
	// r.Respond()
	r.playTextInVoiceChannel(line)
}

func (r *Responder) playTextInVoiceChannel(line string) {
	opusReader, err := r.ttsService.Synthesize(line)
	if err != nil {
		fmt.Printf("Error generating speech: %v", err)
		return
	}
	defer opusReader.Close()

	buf := make([]byte, 4096) // adjust the buffer size if needed
	for {
		n, err := opusReader.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			fmt.Printf("Error reading Opus packet: %s", err)
			return
		}
		r.playAudioChannel <- buf[:n]
	}
}

func (r *Responder) GetTranscript() *transcript.Transcript {
	return r.transcript
}

func (r *Responder) Respond() {
	// Get recent lines of the transcript
	lines := r.transcript.GetRecentLines(10)

	// Create the chat completion stream
	stream, err := llm.GetTranscriptResponseStream(lines, "openai", "gpt-3.5-turbo")
	if err != nil {
		fmt.Printf("Token stream error: %v\n", err)
		return
	}
	defer stream.Close()

	// Initialize a strings.Builder to build sentences from tokens
	var sentenceBuilder strings.Builder

	// Initialize a variable to store the previous token
	var previousToken string

	// Initialize a flag to check if the stream has ended
	streamEnded := false

	// Iterate over tokens received from the stream
	for !streamEnded {
		// Receive a token from the stream
		response, err := stream.Recv()

		// If the stream has ended, set the streamEnded flag to true
		if errors.Is(err, io.EOF) {
			streamEnded = true
		} else if err != nil {
			// If there is an error while receiving the token, close the channel and return
			fmt.Printf("\nStream error: %v\n", err)
			return
		} else {
			// Extract the token from the response
			currentToken := response.Choices[0].Delta.Content

			// If there is a previous token, append it to the sentenceBuilder
			if previousToken != "" {
				sentenceBuilder.WriteString(previousToken)

				// If the previous token ends with a sentence-ending character and the current token starts with a whitespace, emit the sentence and reset the sentenceBuilder
				if isEndOfSentence(previousToken) && startsWithWhitespace(currentToken) {
					sentence := sentenceBuilder.String()
					r.sentenceChan <- sentence
					sentenceBuilder.Reset()
				}
			}

			// Set the previous token to be the current token for the next iteration
			previousToken = currentToken
		}
	}
	// Emit any remaining sentence
	if sentenceBuilder.Len() > 0 {
		r.sentenceChan <- sentenceBuilder.String()
	}
}

// isEndOfSentence checks if a token ends with a sentence-ending character or a sentence-ending character followed by a quote
func isEndOfSentence(token string) bool {
	endChars := []string{".", "!", "?", ";", ":", "-", "\n"}
	quoteChars := []string{"\"", "”", "“", "`", "'"}

	for _, endChar := range endChars {
		// Check if the token ends with any of the end characters
		if strings.HasSuffix(token, endChar) {
			return true
		}

		// Check if the token ends with an end character followed by a quote
		for _, quoteChar := range quoteChars {
			if strings.HasSuffix(token, endChar+quoteChar) {
				return true
			}
		}
	}

	return false
}

// startsWithWhitespace checks if a token starts with a whitespace character
func startsWithWhitespace(token string) bool {
	if len(token) == 0 {
		return false
	}
	firstChar := rune(token[0])
	return unicode.IsSpace(firstChar)
}
