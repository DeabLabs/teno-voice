package transcript

import (
	"strings"
	"time"
)

type Transcript struct {
	lines           []string
}

func NewTranscript() *Transcript {
	return &Transcript{
		lines:           make([]string, 0),
	}
}

func (t *Transcript) AddLine(startTime time.Time, userId string, transcription string) error {
	formattedLine := formatLine(startTime, userId, transcription)
	t.lines = append(t.lines, formattedLine)

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

func formatLine(startTime time.Time, userId string, line string) string {
	return startTime.Format("15:04:05") + " " + userId + ": " + line
}