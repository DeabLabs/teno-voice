package responder

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
	"unicode"

	"com.deablabs.teno-voice/internal/llm"
	"com.deablabs.teno-voice/internal/llm/promptbuilder"

	"com.deablabs.teno-voice/internal/responder/tools"
	texttospeech "com.deablabs.teno-voice/internal/textToSpeech"
	"com.deablabs.teno-voice/internal/transcript"
	"com.deablabs.teno-voice/internal/usage"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/redis/go-redis/v9"
)

type VoiceUXConfig struct {
	SpeakingMode               string `validate:"required"`
	LinesBeforeSleep           int
	BotNameConfidenceThreshold float64
}

type NewResponderArgs struct {
	BotName                string
	PlayAudioChannel       chan []byte
	Conn                   *voice.Conn
	TTSService             *texttospeech.TextToSpeechService
	LLMService             *llm.LLMService
	VoiceUXConfig          VoiceUXConfig
	PromptContents         *promptbuilder.PromptContents
	TranscriptSSEChannel   chan string
	ToolMessagesSSEChannel chan string
	RedisClient            *redis.Client
	RedisTranscriptKey     string
	TranscriptConfig       transcript.TranscriptConfig
	BotId                  snowflake.ID
}

type Responder struct {
	BotName                string
	Transcript             *transcript.Transcript
	playAudioChannel       chan []byte
	conn                   voice.Conn
	TtsService             texttospeech.TextToSpeechService
	LlmService             llm.LLMService
	cancelResponse         context.CancelFunc
	awake                  bool
	linesSinceLastResponse int
	VoiceUXConfig          VoiceUXConfig
	PromptContents         promptbuilder.PromptContents
	botId                  snowflake.ID
	toolMessagesSSEChannel chan string
	isSpeaking             bool
}

type audioStreamWithIndex struct {
	index       int
	opusPackets io.ReadCloser
	sentence    string
}

func NewResponder(args NewResponderArgs) *Responder {
	responder := &Responder{
		BotName:                args.BotName,
		playAudioChannel:       args.PlayAudioChannel,
		conn:                   *args.Conn,
		Transcript:             transcript.NewTranscript(args.TranscriptSSEChannel, args.RedisClient, args.RedisTranscriptKey, args.TranscriptConfig),
		TtsService:             *args.TTSService,
		LlmService:             *args.LLMService,
		VoiceUXConfig:          args.VoiceUXConfig,
		PromptContents:         *args.PromptContents,
		cancelResponse:         nil,
		awake:                  true,
		linesSinceLastResponse: 0,
		botId:                  args.BotId,
		toolMessagesSSEChannel: args.ToolMessagesSSEChannel,
		isSpeaking:             false,
	}

	return responder
}

func (r *Responder) UpdateConfig(newVoiceUxConfig VoiceUXConfig, newPromptContents promptbuilder.PromptContents) {
	if reflect.DeepEqual(r.VoiceUXConfig, newVoiceUxConfig) {
		return
	} else {
		r.VoiceUXConfig = newVoiceUxConfig
	}

	if reflect.DeepEqual(r.PromptContents, newPromptContents) {
		return
	} else {
		r.PromptContents = newPromptContents
	}
}

func (r *Responder) Respond(receivedTranscriptionTime time.Time) context.CancelFunc {
	ctx, cancelFunc := context.WithCancel(context.Background())
	sentenceChan := make(chan string)
	audioStreamChan := make(chan audioStreamWithIndex, 100)

	// Create channel for tool messages ready to be sent
	toolMessageChan := make(chan string, 1)

	// Get recent lines of the transcript
	lines := r.Transcript.GetTranscript()

	wg := sync.WaitGroup{}

	// Start the goroutine to get a stream of tokens
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.getTokenStream(ctx, lines, sentenceChan, toolMessageChan)
	}()

	// Start the goroutine to synthesize the sentences into audio
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.synthesizeSentences(ctx, sentenceChan, audioStreamChan)
	}()

	// Start the goroutine to play the synthesized sentences
	wg.Add(1)
	go func() {
		defer wg.Done()
		r.playSynthesizedSentences(ctx, receivedTranscriptionTime, audioStreamChan)
	}()

	// Start a goroutine to wait for all other goroutines to finish
	go func() {
		wg.Wait()

		// Check if the context was cancelled
		select {
		case <-ctx.Done():
			// If the context was cancelled, don't send the tool message
			return
		default:
			// If the context wasn't cancelled, send the tool message
			for toolMessage := range toolMessageChan {
				r.toolMessagesSSEChannel <- toolMessage
			}
		}

		// After all other goroutines have finished, close the toolMessageChan
		close(toolMessageChan)
	}()

	return cancelFunc
}

func (r *Responder) getTokenStream(ctx context.Context, lines string, sentenceChan chan string, toolMessageChan chan string) {
	// Create the chat completion stream
	stream, usageEvent, err := r.LlmService.GetTranscriptResponseStream(lines, r.BotName, &r.PromptContents)
	if err != nil {
		fmt.Printf("Token stream error: %v\n", err)
		return
	}
	defer stream.Close()
	defer close(sentenceChan)

	var totalTokens int

	// Initialize a strings.Builder to build sentences from tokens
	var sentenceBuilder strings.Builder

	// Add this line to initialize a strings.Builder for tool messages
	var toolMessageBuilder strings.Builder

	// Initialize a variable to store the previous token
	var previousToken string

	// Initialize a flag to check if the stream has ended
	streamEnded := false

	// Initialize a flag to check if we're in the tool message section
	var inToolMessages = false

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

			totalTokens++

			// If token is a "^", return
			if strings.Contains(currentToken, "^") {
				return
			}

			// If token is a "|", we've reached the tool message section
			if strings.Contains(currentToken, "|") {
				inToolMessages = true
				// Don't append this token to the sentence

				sentenceBuilder.WriteString(previousToken)
				// Emit the remaining sentence
				sentenceChan <- sentenceBuilder.String()
				sentenceBuilder.Reset()
				continue
			}

			if inToolMessages {
				// If we're in the tool message section, append the token to the toolMessageBuilder
				toolMessageBuilder.WriteString(currentToken)
			} else if previousToken != "" {
				sentenceBuilder.WriteString(previousToken)

				// If the previous token ends with a sentence-ending character and the current token starts with a whitespace, emit the sentence and reset the sentenceBuilder
				if isEndOfSentence(previousToken) && startsWithWhitespace(currentToken) {
					sentence := sentenceBuilder.String()
					sentenceChan <- sentence
					sentenceBuilder.Reset()
				}
			}

			// Set the previous token to be the current token for the next iteration
			previousToken = currentToken
		}
	}

	// Emit any remaining sentence
	if sentenceBuilder.Len() > 0 {
		sentenceChan <- sentenceBuilder.String()
	}

	// Validate and emit any remaining tool messages
	if toolMessageBuilder.Len() > 0 {
		toolMessage := toolMessageBuilder.String()
		log.Printf("Tool message: %v\n", toolMessage)
		if tools.IsValidToolMessage(toolMessage, r.PromptContents.ToolList) {
			toolMessageChan <- toolMessage
			r.Transcript.AddToolMessageLine(toolMessage)
		} else {
			fmt.Printf("Invalid tool message: %v\n", toolMessage)
		}
	}

	usageEvent.SetCompletionTokens(totalTokens)
	if !usageEvent.IsEmpty() {
		usage.SendEventToDB(usageEvent)
	}
}

func (r *Responder) synthesizeSentences(ctx context.Context, sentenceChan chan string, audioStreamChan chan audioStreamWithIndex) {
	defer close(audioStreamChan) // Make sure to close the audioStreamChan when sentenceChan is closed

	sentenceIndex := 0
	for sentence := range sentenceChan {
		select {
		case <-ctx.Done():
			return
		default:
		}
		opusPackets, err := r.TtsService.Synthesize(sentence)
		if err != nil {
			fmt.Printf("Error generating speech: %v\n", err)
			continue
		}
		audioStreamChan <- audioStreamWithIndex{
			index:       sentenceIndex,
			opusPackets: opusPackets,
			sentence:    sentence,
		}
		sentenceIndex++
	}
}

func (r *Responder) playSynthesizedSentences(ctx context.Context, receivedTranscriptionTime time.Time, audioStreamChan chan audioStreamWithIndex) {
	audioStreamMap := make(map[int]io.ReadCloser)
	nextAudioIndex := 0
	bytesToDiscard := 1700 // Adjust this value based on how much you want to trim from the beginning

	firstSentence := true

	for audioStreamWithIndex := range audioStreamChan {
		audioStreamMap[audioStreamWithIndex.index] = audioStreamWithIndex.opusPackets
		sentence := audioStreamWithIndex.sentence

		r.setSpeaking(true)

		if firstSentence {
			r.isSpeaking = true
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
					r.isSpeaking = false
					r.sendSilentFrames(5)
					r.setSpeaking(false)
					opusPackets.Close()
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
	r.isSpeaking = false
}

func (r *Responder) NewTranscription(line string, botNameSpoken float64, username string, userId string) {
	receivedTranscriptionTime := time.Now()

	newLine := &transcript.Line{
		Text:     line,
		Username: username,
		UserId:   userId,
		Time:     time.Now(),
	}

	r.Transcript.AddSpokenLine(newLine)
	r.linesSinceLastResponse++

	if r.cancelResponse != nil {
		r.cancelResponse()
	}

	switch r.VoiceUXConfig.SpeakingMode {
	case "NeverSpeak":
		return
	case "AlwaysSleep":
		r.awake = false
	case "AutoSleep":
		if r.linesSinceLastResponse > r.VoiceUXConfig.LinesBeforeSleep {
			r.awake = false
			// log.Printf("Bot is asleep\n")
		}

		if botNameSpoken > r.VoiceUXConfig.BotNameConfidenceThreshold {
			r.awake = true
			r.linesSinceLastResponse = 0
			// log.Printf("Bot is awake\n")
		}
	default: // AlwaysSpeak
		r.awake = true
	}

	// Only respond if the bot is awake
	if r.awake {
		if r.isSpeaking {
			r.Transcript.AddInterruptionLine(username, r.BotName)
		}
		r.cancelResponse = r.Respond(receivedTranscriptionTime)
	}
}

func (r *Responder) botLineSpoken(line string) {
	newLine := &transcript.Line{
		Text:     line,
		Username: r.BotName,
		UserId:   r.botId.String(),
		Time:     time.Now(),
	}
	r.Transcript.AddSpokenLine(newLine)

	// Reset the counter when the bot speaks
	r.linesSinceLastResponse = 0
}

func (r *Responder) InterimTranscriptionReceived() {
	if r.cancelResponse != nil {
		r.cancelResponse()
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
