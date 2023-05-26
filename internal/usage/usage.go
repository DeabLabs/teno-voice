package usage

import "encoding/json"

type UsageEvent interface {
	UsageType() string
}

// usageEventToJSON converts a UsageEvent to a JSON string
func UsageEventToJSON(event UsageEvent) (string, error) {
	jsonEvent, err := json.Marshal(event)
	if err != nil {
		return "", err
	}

	return string(jsonEvent), nil
}

type TextToSpeechEvent struct {
	Service    string
	Model      string
	Characters int
	// other common fields...
}

func NewTextToSpeechEvent(service string, model string, characters int) *TextToSpeechEvent {
	return &TextToSpeechEvent{Service: service, Model: model, Characters: characters}
}

func (t TextToSpeechEvent) UsageType() string {
	return "TextToSpeech"
}

func (t *TextToSpeechEvent) IsEmpty() bool {
	return *t == TextToSpeechEvent{}
}

type TranscriptionEvent struct {
	Service string
	Model   string
	Minutes float64
	// other common fields...
}

func NewTranscriptionEvent(service string, model string, minutes float64) *TranscriptionEvent {
	return &TranscriptionEvent{Minutes: minutes}
}

func (t TranscriptionEvent) UsageType() string {
	return "Transcription"
}

func (t *TranscriptionEvent) IsEmpty() bool {
	return *t == TranscriptionEvent{}
}

type LLMEvent struct {
	Service          string
	Model            string
	PromptTokens     int
	CompletionTokens int
	// other common fields...
}

func NewLLMEvent(service string, model string, promptTokens int, completionTokens int) *LLMEvent {
	return &LLMEvent{Service: service, Model: model, PromptTokens: promptTokens, CompletionTokens: completionTokens}
}

func (l *LLMEvent) SetCompletionTokens(tokens int) {
	l.CompletionTokens = tokens
}

func (l *LLMEvent) IsEmpty() bool {
	return *l == LLMEvent{}
}

func (l LLMEvent) UsageType() string {
	return "LLM"
}
