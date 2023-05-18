package usage

type UsageEvent interface {
	UsageType() string
}

func SendEventToDB(event UsageEvent) {
	// //switch e := event.(type) {
	// case *TextToSpeechEvent:
	// 	//log.Printf("TTS event with %d characters", e.Characters)
	// case *TranscriptionEvent:
	// 	//log.Printf("Transcription event with %0.2f minutes", e.Minutes)
	// case *LLMEvent:
	// 	//log.Printf("LLM event with %d prompt tokens and %d completion tokens", e.PromptTokens, e.CompletionTokens)
	// default:
	// 	//log.Printf("Unknown event")
	// }
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

func (t *TextToSpeechEvent) UsageType() string {
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

func (t *TranscriptionEvent) UsageType() string {
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

func (l *LLMEvent) UsageType() string {
	return "LLM"
}
