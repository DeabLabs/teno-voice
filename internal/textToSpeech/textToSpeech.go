package texttospeech

import "io"

type TextToSpeechService interface {
	Synthesize(text string) (<-chan []byte, error)
}

type ReadCloserWrapper struct {
	io.Reader
	Closer func() error
}

func (w *ReadCloserWrapper) Close() error {
	return w.Closer()
}
