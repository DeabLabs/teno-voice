package responder

import (
	// "context"
	// "errors"

	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"strings"
	"sync"
	"time"
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
	opusPackets io.ReadCloser
	sentence    string
}

type Responder struct {
	transcript        *transcript.Transcript
	playAudioChannel  chan []byte
	audioStreamChan   chan audioStreamWithIndex
	speakingStateChan chan UserSpeakingState
	speakingUsers     map[snowflake.ID]struct{}
	speakingUsersMu   sync.Mutex
	isAnyUserSpeaking bool
	sentenceChan      chan string
	ttsService        texttospeech.TextToSpeechService
	cancelResponse    context.CancelFunc
}

func NewResponder(playAudioChannel chan []byte, ttsService texttospeech.TextToSpeechService) *Responder {
	responder := &Responder{
		playAudioChannel:  playAudioChannel,
		sentenceChan:      make(chan string, 100),
		audioStreamChan:   make(chan audioStreamWithIndex, 100),
		transcript:        transcript.NewTranscript(),
		speakingStateChan: make(chan UserSpeakingState),
		speakingUsers:     make(map[snowflake.ID]struct{}),
		isAnyUserSpeaking: false,
		ttsService:        ttsService,
		cancelResponse:    nil,
	}

	return responder
}

func (r *Responder) UpdateUserSpeakingState(userID snowflake.ID, isSpeaking bool) {
	r.speakingUsersMu.Lock()
	if isSpeaking {
		r.speakingUsers[userID] = struct{}{}
	} else {
		delete(r.speakingUsers, userID)
	}

	r.isAnyUserSpeaking = len(r.speakingUsers) > 0
	r.speakingUsersMu.Unlock()

	log.Printf("Speaking users: %v\n", r.speakingUsers)
	log.Printf("isAnyUserSpeaking: %v\n", r.isAnyUserSpeaking)
}

func (r *Responder) synthesizeSentences(ctx context.Context) {
	defer close(r.audioStreamChan) // Make sure to close the audioStreamChan when sentenceChan is closed

	sentenceIndex := 0
	for sentence := range r.sentenceChan {
		log.Printf("Synthesizing sentence: %s\n", sentence)
		opusPackets, err := r.ttsService.Synthesize(sentence)
		if err != nil {
			fmt.Printf("Error generating speech: %v\n", err)
			continue
		}
		r.audioStreamChan <- audioStreamWithIndex{
			index:       sentenceIndex,
			opusPackets: opusPackets,
			sentence:    sentence,
		}
		sentenceIndex++
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
}

func (r *Responder) playSynthesizedSentences(ctx context.Context) {
	audioStreamMap := make(map[int]io.ReadCloser)
	nextAudioIndex := 0
	bytesToDiscard := 1000 // Adjust this value based on how much you want to trim from the beginning

	for audioStreamWithIndex := range r.audioStreamChan {
		audioStreamMap[audioStreamWithIndex.index] = audioStreamWithIndex.opusPackets
		sentence := audioStreamWithIndex.sentence

		for {
			opusPackets, ok := audioStreamMap[nextAudioIndex]
			if !ok {
				break
			}

			// Discard bytes from the beginning of the audio stream
			if err := r.discardBytes(opusPackets, bytesToDiscard); err != nil {
				fmt.Printf("Error discarding bytes: %s\n", err)
				break
			}

			// Use a buffer to read the packets and send them to the playAudioChannel
			buf := make([]byte, 4096)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				n, err := opusPackets.Read(buf)
				if err != nil {
					if err != io.EOF {
						fmt.Printf("Error reading Opus packet: %s\n", err)
					}
					break
				}
				r.playAudioChannel <- buf[:n]
			}
			opusPackets.Close() // Close the opusPackets after playing

			r.botLineSpoken(sentence)

			// Remove the played audio stream from the map and increment the nextAudioIndex
			delete(audioStreamMap, nextAudioIndex)
			nextAudioIndex++
		}
	}
}

func (r *Responder) InterimTranscriptionReceived() {
	if r.cancelResponse != nil {
		r.cancelResponse()
	}
}

func (r *Responder) NewTranscription(line string) {
	formattedLine := r.FormatLine("User", line, time.Now())
	r.transcript.AddLine(formattedLine)

	if r.cancelResponse != nil {
		r.cancelResponse()
	}

	r.cancelResponse = r.Respond()
}

func (r *Responder) botLineSpoken(line string) {
	formattedLine := r.FormatLine("Bot", line, time.Now())
	r.transcript.AddLine(formattedLine)
}

func (r *Responder) Respond() context.CancelFunc {
	ctx, cancelFunc := context.WithCancel(context.Background())
	r.sentenceChan = make(chan string)
	r.audioStreamChan = make(chan audioStreamWithIndex)

	// Create channel to signal end of token stream
	// tokenStreamDone := make(chan struct{})

	// Get recent lines of the transcript
	lines := r.transcript.GetRecentLines(10)

	// Start the goroutine to get a stream of tokens
	go r.getTokenStream(ctx, lines, "openai", "gpt-3.5-turbo")

	// Start the goroutine to synthesize the sentences into audio
	go r.synthesizeSentences(ctx)

	// Start the goroutine to play the synthesized sentences
	go r.playSynthesizedSentences(ctx)

	return cancelFunc
}

func (r *Responder) getTokenStream(ctx context.Context, lines string, service string, model string) {
	// Create the chat completion stream
	stream, err := llm.GetTranscriptResponseStream(lines, service, model)
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

			// If token is a "^", return
			if currentToken == "^" {
				return
			}

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
		select {
		case <-ctx.Done():
			return
		default:
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

func (r *Responder) discardBytes(reader io.Reader, bytesToDiscard int) error {
	buf := make([]byte, 4096)

	for bytesToDiscard > 0 {
		n := int(math.Min(float64(cap(buf)), float64(bytesToDiscard)))
		readBytes, err := reader.Read(buf[:n])
		if err != nil {
			return err
		}
		bytesToDiscard -= readBytes
	}

	return nil
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

// Format the line for the transcript, including the username, the line spoken, and the human readable timestamp
func (r *Responder) FormatLine(username string, line string, timestamp time.Time) string {
	return fmt.Sprintf("[%s] %s: %s", timestamp.Format("15:04:05"), username, line)
}
