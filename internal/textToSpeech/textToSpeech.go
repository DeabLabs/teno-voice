package texttospeech

import "io"

type TextToSpeechService interface {
	Synthesize(text string) (io.ReadCloser, error)
}
