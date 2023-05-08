package transcript

import (
	"strings"
)

type Transcript struct {
	lines                []string
	transcriptSSEChannel chan string
}

func NewTranscript(transcriptSSEChannel chan string) *Transcript {
	return &Transcript{
		lines:                make([]string, 0),
		transcriptSSEChannel: transcriptSSEChannel,
	}
}

func (t *Transcript) AddLine(line string) error {
	t.lines = append(t.lines, line)

	select {
	case t.transcriptSSEChannel <- line:
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
