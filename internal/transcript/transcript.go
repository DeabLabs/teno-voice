package transcript

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	goOpenai "github.com/sashabaranov/go-openai"
)

type Transcript struct {
	lines                []Line
	transcriptSSEChannel chan string
	redisClient          redis.Client
	transcriptKey        string
	Config               TranscriptConfig
	mu                   sync.Mutex
}

type TranscriptConfig struct {
	NumberOfTranscriptLines int `validate:"required"`
}

type Line struct {
	Text          string
	FormattedText string
	Username      string
	UserId        string
	Type          string
	Time          time.Time
}

func NewTranscript(transcriptSSEChannel chan string, redisClient *redis.Client, transcriptKey string, config TranscriptConfig) *Transcript {
	return &Transcript{
		lines:                make([]Line, 0),
		transcriptSSEChannel: transcriptSSEChannel,
		redisClient:          *redisClient,
		transcriptKey:        transcriptKey,
		Config:               config,
	}
}

func (t *Transcript) ClearTranscript() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lines = make([]Line, 0)
}

func (t *Transcript) Cleanup() {
	t.ClearTranscript()
	close(t.transcriptSSEChannel)
}

func (t *Transcript) addLine(line *Line) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// If the slice has reached the max limit from config, remove the oldest element before appending.
	if len(t.lines) >= t.Config.NumberOfTranscriptLines {
		t.lines = t.lines[1:]
	}

	t.lines = append(t.lines, *line)
	log.Printf("Transcript Line: %s", line.FormattedText)
}

func (t *Transcript) AddSpokenLine(line *Line) error {
	line.FormattedText = formatLine(*line)

	t.addLine(line)

	if t.transcriptKey != "" {
		go func() {
			redisText := formatForRedis(*line, line.FormattedText)
			err := t.SendLineToRedis(*line, redisText)
			if err != nil {
				fmt.Printf("SendLineToRedis error: %v\n", err)
			}
		}()
	}

	select {
	case t.transcriptSSEChannel <- line.FormattedText:
	default:
	}

	return nil
}

func (t *Transcript) AddInterruptionLine(username string, botName string) {
	text := fmt.Sprintf("[%s interrupted %s]", username, botName)

	newLine := &Line{
		Text:     text,
		Username: "",
		UserId:   "",
		Type:     "system",
		Time:     time.Now(),
	}

	t.addLine(newLine)
}

func (t *Transcript) AddTaskReminderLine(task string) {
	text := "Complete the task: " + task

	newLine := &Line{
		Text:     text,
		Username: "",
		UserId:   "",
		Type:     "system",
		Time:     time.Now(),
	}

	t.addLine(newLine)
}

func (t *Transcript) AddNewDocumentAlertLine() {
	text := "New document available, please relay the relevant information to the voice channel"

	newLine := &Line{
		Text:     text,
		Username: "",
		UserId:   "",
		Type:     "system",
		Time:     time.Now(),
	}

	t.addLine(newLine)
}

func (t *Transcript) AddToolMessageLine(toolMessage string) {
	text := fmt.Sprintf("|%s", toolMessage)

	newLine := &Line{
		Text:     text,
		Username: "",
		UserId:   "",
		Type:     "assistant",
		Time:     time.Now(),
	}

	t.addLine(newLine)
}

func (t *Transcript) GetTranscriptString() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	var lineTexts []string
	for _, line := range t.lines {
		lineTexts = append(lineTexts, line.Text)
	}

	return strings.Join(lineTexts, "\n")
}

func (t *Transcript) GetTranscript() []Line {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.lines
}

func (t *Transcript) ToChatCompletionMessages() []goOpenai.ChatCompletionMessage {
	t.mu.Lock()
	defer t.mu.Unlock()

	var messages []goOpenai.ChatCompletionMessage
	assistantBuffer := ""

	for i, line := range t.lines {
		var role string
		var content string
		switch line.Type {
		case "system":
			role = goOpenai.ChatMessageRoleSystem
			content = line.Text
		case "assistant":
			role = goOpenai.ChatMessageRoleAssistant
			assistantBuffer += line.Text + " "
			// If the next line is not from assistant or it is the last line, create a message from buffer
			if i == len(t.lines)-1 || t.lines[i+1].Type != "assistant" {
				messages = append(messages, goOpenai.ChatCompletionMessage{
					Role:    role,
					Content: strings.TrimSpace(assistantBuffer),
				})
				assistantBuffer = ""
			}
			continue
		default:
			role = goOpenai.ChatMessageRoleUser
			content = line.Username + ": " + line.Text
		}
		messages = append(messages, goOpenai.ChatCompletionMessage{
			Role:    role,
			Content: content,
		})
	}
	return messages
}

func (t *Transcript) SendLineToRedis(line Line, formattedLine string) error {
	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result, err := t.redisClient.ZAdd(ctx, t.transcriptKey, redis.Z{
		Score:  float64(line.Time.UnixMilli()),
		Member: formattedLine,
	}).Result()

	if err != nil {
		return err
	}

	if result == 0 {
		return errors.New("could not append transcript")
	}

	return nil
}

// Format the line for the transcript, including the username, the line spoken, and the human readable timestamp
func formatLine(line Line) string {
	return fmt.Sprintf("[%s] %s: %s", line.Time.Format("15:04:05"), line.Username, strings.TrimSpace(line.Text))
}

// Format should be <userId>formattedText<timestamp in float64>
func formatForRedis(line Line, formattedText string) string {
	return fmt.Sprintf("<%s>%s<%s>", line.UserId, formattedText, strconv.Itoa(int(line.Time.UnixMilli())))
}
