package utterance

import "time"

type Utterance struct {
	transcription string
	userId string
	startTime time.Time
}

func NewUtterance(userId string) *Utterance {
	return &Utterance{
		userId: userId,
		startTime: time.Now(),
	}
}

func (u *Utterance) SetTranscription(transcription string) {
	u.transcription = transcription
}

func (u *Utterance) GetTranscription() string {
	return u.transcription
}

func (u *Utterance) GetUserId() string {
	return u.userId
}

func (u *Utterance) GetStartTime() time.Time {
	return u.startTime
}

func (u *Utterance) GetTimeSinceStart() time.Duration {
	return time.Since(u.startTime)
}