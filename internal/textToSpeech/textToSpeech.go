package texttospeech

import "io"

// TextToSpeech is an interface for text to speech services, it has a method GenerateSpeech which returns a stream of opus packets
type TextToSpeech interface {
	GenerateSpeech(text string) (io.ReadCloser, error)
}
