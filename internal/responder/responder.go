package responder

import (
	// "context"
	// "errors"

	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strings"
	"time"
	"unicode"

	// "strings"

	// "com.deablabs.teno-voice/internal/llm"
	"com.deablabs.teno-voice/internal/llm"
	texttospeech "com.deablabs.teno-voice/internal/textToSpeech"
	"com.deablabs.teno-voice/internal/transcript"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/redis/go-redis/v9"
)

type SleepModeType int

const (
	Unspecified SleepModeType = iota
	AlwaysSleep
	AutoSleep
	NeverSleep
)

type ResponderConfig struct {
	BotName                    string
	Personality                string
	SleepMode                  SleepModeType
	LinesBeforeSleep           int
	BotNameConfidenceThreshold float64
	LLMService                 string
	LLMModel                   string
	TranscriptContextSize      int
}

type audioStreamWithIndex struct {
	index       int
	opusPackets io.ReadCloser
	sentence    string
}

type Responder struct {
	transcript             *transcript.Transcript
	playAudioChannel       chan []byte
	conn                   voice.Conn
	audioStreamChan        chan audioStreamWithIndex
	sentenceChan           chan string
	ttsService             texttospeech.TextToSpeechService
	cancelResponse         context.CancelFunc
	awake                  bool
	linesSinceLastResponse int
	responderConfig        ResponderConfig
	botId                  snowflake.ID
}

func NewResponder(playAudioChannel chan []byte, conn *voice.Conn, ttsService texttospeech.TextToSpeechService, config ResponderConfig, transcriptSSEChannel chan string, redisClient *redis.Client, redisTranscriptKey string, botId snowflake.ID) *Responder {
	responder := &Responder{
		playAudioChannel:       playAudioChannel,
		conn:                   *conn,
		sentenceChan:           make(chan string),
		audioStreamChan:        make(chan audioStreamWithIndex, 100),
		transcript:             transcript.NewTranscript(transcriptSSEChannel, redisClient, redisTranscriptKey),
		ttsService:             ttsService,
		cancelResponse:         nil,
		awake:                  true,
		linesSinceLastResponse: 0,
		responderConfig:        config,
		botId:                  botId,
	}

	return responder
}

func (r *Responder) GetBotName() string {
	return r.responderConfig.BotName
}

func (r *Responder) SetSleepMode(mode SleepModeType) {
	r.responderConfig.SleepMode = mode
}

func (r *Responder) synthesizeSentences(ctx context.Context) {
	defer close(r.audioStreamChan) // Make sure to close the audioStreamChan when sentenceChan is closed

	sentenceIndex := 0
	for sentence := range r.sentenceChan {
		select {
		case <-ctx.Done():
			return
		default:
		}
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
	}
}

func (r *Responder) playSynthesizedSentences(ctx context.Context, receivedTranscriptionTime time.Time) {
	audioStreamMap := make(map[int]io.ReadCloser)
	nextAudioIndex := 0
	bytesToDiscard := 1700 // Adjust this value based on how much you want to trim from the beginning

	firstSentence := true

	for audioStreamWithIndex := range r.audioStreamChan {
		audioStreamMap[audioStreamWithIndex.index] = audioStreamWithIndex.opusPackets
		sentence := audioStreamWithIndex.sentence

		r.setSpeaking(true)

		if firstSentence {
			transcriptionToResponseLatency := time.Since(receivedTranscriptionTime)
			// Print latency in milliseconds
			fmt.Printf("Transcription to response latency: %.0f ms\n", transcriptionToResponseLatency.Seconds()*1000)
			firstSentence = false
		}

		for {
			opusPackets, ok := audioStreamMap[nextAudioIndex]
			if !ok {
				break
			}

			// Discard bytes from the beginning of the audio stream
			if err := r.discardBytes(opusPackets, bytesToDiscard); err != nil {
				break
			}

			// Use a buffer to read the packets and send them to the playAudioChannel
			buf := make([]byte, 8192)
			for {
				select {
				case <-ctx.Done():
					r.sendSilentFrames(5)
					r.setSpeaking(false)
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

				// Send the payload to the playAudioChannel
				r.playAudioChannel <- buf[:n]
			}
			opusPackets.Close() // Close the opusPackets after playing

			r.botLineSpoken(sentence)

			// Remove the played audio stream from the map and increment the nextAudioIndex
			delete(audioStreamMap, nextAudioIndex)
			nextAudioIndex++
		}
		r.sendSilentFrames(1)
		r.setSpeaking(false)
	}
}

func (r *Responder) InterimTranscriptionReceived() {
	if r.cancelResponse != nil {
		r.cancelResponse()
	}
}

func (r *Responder) NewTranscription(line string, botNameSpoken float64, username string, userId string) {
	receivedTranscriptionTime := time.Now()

	newLine := &transcript.Line{
		Text:     line,
		Username: username,
		UserId:   userId,
		Time:     time.Now(),
	}

	r.transcript.AddLine(newLine)
	r.linesSinceLastResponse++

	if r.cancelResponse != nil {
		r.cancelResponse()
	}

	switch r.responderConfig.SleepMode {
	case AlwaysSleep:
		r.awake = false
	case AutoSleep:
		if r.linesSinceLastResponse > r.responderConfig.LinesBeforeSleep {
			r.awake = false
			// log.Printf("Bot is asleep\n")
		}

		if botNameSpoken > r.responderConfig.BotNameConfidenceThreshold {
			r.awake = true
			r.linesSinceLastResponse = 0
			// log.Printf("Bot is awake\n")
		}
	default:
		r.awake = true
	}

	// Only respond if the bot is awake
	if r.awake {
		r.cancelResponse = r.Respond(receivedTranscriptionTime)
	}
}

func (r *Responder) botLineSpoken(line string) {
	newLine := &transcript.Line{
		Text:     line,
		Username: r.responderConfig.BotName,
		UserId:   r.botId.String(),
		Time:     time.Now(),
	}
	r.transcript.AddLine(newLine)

	// Reset the counter when the bot speaks
	r.linesSinceLastResponse = 0
}

func (r *Responder) Respond(receivedTranscriptionTime time.Time) context.CancelFunc {
	ctx, cancelFunc := context.WithCancel(context.Background())
	r.sentenceChan = make(chan string)
	r.audioStreamChan = make(chan audioStreamWithIndex)

	// Get recent lines of the transcript
	lines := r.transcript.GetRecentLines(r.responderConfig.TranscriptContextSize)

	// Start the goroutine to get a stream of tokens
	go r.getTokenStream(ctx, lines)

	// Start the goroutine to synthesize the sentences into audio
	go r.synthesizeSentences(ctx)

	// Start the goroutine to play the synthesized sentences
	go r.playSynthesizedSentences(ctx, receivedTranscriptionTime)

	return cancelFunc
}

func (r *Responder) getTokenStream(ctx context.Context, lines string) {
	// Create the chat completion stream
	stream, err := llm.GetTranscriptResponseStream(lines, r.responderConfig.LLMService, r.responderConfig.LLMModel, r.GetBotName(), r.responderConfig.Personality)
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
	buf := make([]byte, 8192)

	// If the reader is a net.Conn, set a read deadline
	if conn, ok := reader.(net.Conn); ok {
		deadline := time.Now().Add(5 * time.Second) // Adjust the timeout as needed
		conn.SetReadDeadline(deadline)
	}

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

func (r *Responder) GetTranscript() *transcript.Transcript {
	return r.transcript
}

func (r *Responder) Configure(newConfig ResponderConfig) error {
	if newConfig.BotName != "" {
		r.responderConfig.BotName = newConfig.BotName
	}

	if newConfig.Personality != "" {
		r.responderConfig.Personality = newConfig.Personality
	}

	if newConfig.SleepMode != 0 {
		r.responderConfig.SleepMode = newConfig.SleepMode
	}

	if newConfig.LinesBeforeSleep != 0 {
		r.responderConfig.LinesBeforeSleep = newConfig.LinesBeforeSleep
	}

	if newConfig.BotNameConfidenceThreshold != 0 {
		r.responderConfig.BotNameConfidenceThreshold = newConfig.BotNameConfidenceThreshold
	}

	if newConfig.LLMService != "" {
		r.responderConfig.LLMService = newConfig.LLMService
	}

	if newConfig.LLMModel != "" {
		r.responderConfig.LLMModel = newConfig.LLMModel
	}

	if newConfig.TranscriptContextSize != 0 {
		r.responderConfig.TranscriptContextSize = newConfig.TranscriptContextSize
	}

	return nil
}

func (r *Responder) setSpeaking(speaking bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if speaking {
		err := r.conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone)
		if err != nil {
			fmt.Printf("Error setting speaking on: %s\n", err)
		}
	} else {
		err := r.conn.SetSpeaking(ctx, voice.SpeakingFlagNone)
		if err != nil {
			fmt.Printf("Error setting speaking off: %s\n", err)
		}
	}
}

func (r *Responder) sendSilentFrames(frames int) {
	// Define silence Opus frame
	silenceOpusFrame := []byte{0xF8, 0xFF, 0xFE}

	// Send silent frames after finishing each sentence
	for i := 0; i < frames; i++ {
		// Send the payload to the playAudioChannel
		r.playAudioChannel <- silenceOpusFrame
	}
}
