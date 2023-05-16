package texttospeech

import (
	"io"

	"com.deablabs.teno-voice/internal/usage"
)

type TextToSpeechService interface {
	Synthesize(text string) (io.ReadCloser, *usage.TextToSpeechEvent, error)
}
