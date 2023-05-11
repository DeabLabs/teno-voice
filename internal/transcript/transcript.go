package transcript

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Transcript struct {
	lines                []string
	transcriptSSEChannel chan string
	redisClient          redis.Client
	transcriptKey        string
}

type Line struct {
	Text     string
	Username string
	UserId   string
	Time     time.Time
}

func NewTranscript(transcriptSSEChannel chan string, redisClient *redis.Client, transcriptKey string) *Transcript {
	return &Transcript{
		lines:                make([]string, 0),
		transcriptSSEChannel: transcriptSSEChannel,
		redisClient:          *redisClient,
		transcriptKey:        transcriptKey,
	}
}

func (t *Transcript) AddLine(line *Line) error {
	formattedText := formatLine(*line)
	t.lines = append(t.lines, formattedText)

	go func() {
		redisText := formatForRedis(*line, formattedText)
		err := t.SendLineToRedis(*line, redisText)
		if err != nil {
			fmt.Printf("SendLineToRedis error: %v\n", err)
		}
	}()

	select {
	case t.transcriptSSEChannel <- formattedText:
	default:
	}

	return nil
}

func (t *Transcript) GetTranscript() []string {
	return t.lines
}

// Get recent lines as a string separated by newlines
func (t *Transcript) GetRecentLines(numLines int) string {
	if numLines > len(t.lines) {
		numLines = len(t.lines)
	}

	lines := t.lines[len(t.lines)-numLines:]
	return strings.Join(lines, "\n")
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
