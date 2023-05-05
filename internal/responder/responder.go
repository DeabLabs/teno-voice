package responder

import (
	// "context"
	// "errors"

	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"unicode"

	// "strings"

	// "com.deablabs.teno-voice/internal/llm"
	"com.deablabs.teno-voice/internal/llm"
	"com.deablabs.teno-voice/internal/textToSpeech/azure"
	"com.deablabs.teno-voice/internal/transcript"
	"github.com/disgoorg/snowflake/v2"
	"mccoy.space/g/ogg"
)

type UserSpeakingState struct {
	UserID     snowflake.ID
	IsSpeaking bool
}

type audioStreamWithIndex struct {
	index       int
	audioStream *azure.ReadCloserWrapper
}

type Responder struct {
	transcript        *transcript.Transcript
	playAudioChannel  chan []byte
	audioStreamChan   chan audioStreamWithIndex
	speakingStateChan chan UserSpeakingState
	speakingUsers     map[snowflake.ID]bool
	speakingUsersMu   sync.Mutex
	sentenceChan      chan string
}

func NewResponder(playAudioChannel chan []byte) *Responder {
	responder := &Responder{
		playAudioChannel:  playAudioChannel,
		sentenceChan:      make(chan string, 100),
		audioStreamChan:   make(chan audioStreamWithIndex, 100),
		transcript:        transcript.NewTranscript(),
		speakingStateChan: make(chan UserSpeakingState),
		speakingUsers:     make(map[snowflake.ID]bool),
	}

	go responder.listenForSpeakingState()
	go responder.synthesizeSentences()
	go responder.playSynthesizedSentences()

	return responder
}

func (r *Responder) UpdateUserSpeakingState(userID snowflake.ID, isSpeaking bool) {
	r.speakingStateChan <- UserSpeakingState{
		UserID:     userID,
		IsSpeaking: isSpeaking,
	}
}

func (r *Responder) synthesizeSentences() {
	sentenceIndex := 0
	for sentence := range r.sentenceChan {
		log.Printf("Synthesizing sentence: %s\n", sentence)
		audioReader, err := azure.TextToSpeech(sentence)
		if err != nil {
			fmt.Printf("Error generating speech: %v\n", err)
			continue
		}
		r.audioStreamChan <- audioStreamWithIndex{
			index:       sentenceIndex,
			audioStream: audioReader,
		}
		sentenceIndex++
	}
}

func (r *Responder) playSynthesizedSentences() {
	audioStreamMap := make(map[int]*azure.ReadCloserWrapper)
	nextAudioIndex := 0

	for audioStreamWithIndex := range r.audioStreamChan {
		audioStreamMap[audioStreamWithIndex.index] = audioStreamWithIndex.audioStream

		for {
			audioReader, ok := audioStreamMap[nextAudioIndex]
			if !ok {
				break
			}

			// Create a new Ogg Decoder
			decoder := ogg.NewDecoder(audioReader.Reader)

			for {
				// Decode Ogg pages
				page, err := decoder.Decode()
				if err != nil {
					if err == io.EOF {
						break
					}
					fmt.Printf("Error decoding Ogg page: %s\n", err)
					return
				}

				// Extract raw Opus packets from each page and send them to the playAudioChannel
				for _, packet := range page.Packets {
					r.playAudioChannel <- packet
				}
			}

			// Close the audio stream
			audioReader.Close()

			// Remove the played audio stream from the map and increment the nextAudioIndex
			delete(audioStreamMap, nextAudioIndex)
			nextAudioIndex++
		}
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
	r.Respond()
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
