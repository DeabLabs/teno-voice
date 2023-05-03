package transcript

import (
	azure "com.deablabs.teno-voice/internal/textToSpeech/azure"
)

type Transcript struct {
	lines           []string
	audioBytesChannel chan []byte
}

func NewTranscript(audioBytesChannel chan []byte) *Transcript {
	return &Transcript{
		lines:           make([]string, 0),
		audioBytesChannel: audioBytesChannel,
	}
}

func (t *Transcript) AddLine(line string) error {
	t.lines = append(t.lines, line)
	audioBytes, err := azure.TextToSpeech(line)
	if err != nil {
		return err
	}

	t.audioBytesChannel <- audioBytes
	return nil
}

func (t *Transcript) GetTranscript() []string {
	return t.lines
}