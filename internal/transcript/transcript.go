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
)

type Transcript struct {
	lines                []string
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
	Text     string
	Username string
	UserId   string
	Time     time.Time
}

func NewTranscript(transcriptSSEChannel chan string, redisClient *redis.Client, transcriptKey string, config TranscriptConfig) *Transcript {
	return &Transcript{
		lines:                make([]string, 0),
		transcriptSSEChannel: transcriptSSEChannel,
		redisClient:          *redisClient,
		transcriptKey:        transcriptKey,
		Config:               config,
	}
}

func (t *Transcript) addLine(formattedText string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// If the slice has reached the max limit from config, remove the oldest element before appending.
	if len(t.lines) >= t.Config.NumberOfTranscriptLines {
		t.lines = t.lines[1:]
	}
	t.lines = append(t.lines, formattedText)
}

func (t *Transcript) AddSpokenLine(line *Line) error {
	formattedText := formatLine(*line)
	t.addLine(formattedText)

	if t.transcriptKey != "" {
		go func() {
			redisText := formatForRedis(*line, formattedText)
			err := t.SendLineToRedis(*line, redisText)
			if err != nil {
				fmt.Printf("SendLineToRedis error: %v\n", err)
			}
		}()
	}

	select {
	case t.transcriptSSEChannel <- formattedText:
	default:
	}

	return nil
}

func (t *Transcript) AddInterruptionLine(username string, botName string) {
	line := fmt.Sprintf("[%s interrupted %s]", username, botName)
	t.addLine(line)
}

func (t *Transcript) AddTaskReminderLine() {
	line := "[Tasks pending]"
	t.addLine(line)
}

func (t *Transcript) AddToolMessageLine(toolMessage string) {
	toolMessageLine := fmt.Sprintf("|%s", toolMessage)
	t.addLine(toolMessageLine)
}

// Get lines as a string separated by newlines
func (t *Transcript) GetTranscript() string {
	return strings.Join(t.lines, "\n")
}

func (t *Transcript) SendLineToRedis(line Line, formattedLine string) error {
	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log.Printf("Line to send to redis: %s\n", formattedLine)

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
