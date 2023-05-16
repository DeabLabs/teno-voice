package elevenlabs

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	// Config "com.deablabs.teno-voice/internal/config"
)

// var apiKey = Config.Environment.ElevenLabsToken
var apiKey = ""

type VoiceSettings struct {
	Stability       float64 `json:"stability"`
	SimilarityBoost float64 `json:"similarity_boost"`
	VoiceId         string  `json:"voice_id"`
}

type TTSRequest struct {
	Text          string        `json:"text"`
	ModelID       string        `json:"model_id"`
	VoiceSettings VoiceSettings `json:"voice_settings"`
}

type ElevenLabsTTS struct {
	VoiceSettings            VoiceSettings
	TTSRequest               TTSRequest
	OptimizeStreamingLatency int
}

// We will need to use ffmpeg to convert from mp3 to opus
func (e *ElevenLabsTTS) Synthesize(text string) (io.ReadCloser, error) {
	// Create a TTSRequest
	ttsReq := TTSRequest{
		Text:    text,
		ModelID: e.TTSRequest.ModelID,
		VoiceSettings: VoiceSettings{
			Stability:       e.VoiceSettings.Stability,
			SimilarityBoost: e.VoiceSettings.SimilarityBoost,
			VoiceId:         e.TTSRequest.VoiceSettings.VoiceId,
		},
	}

	// Encode the TTSRequest to JSON
	jsonReq, err := json.Marshal(ttsReq)
	if err != nil {
		return nil, err
	}

	// Create the base URL
	baseUrl, err := url.Parse("https://api.elevenlabs.io/v1/text-to-speech/" + e.VoiceSettings.VoiceId + "/stream")
	if err != nil {
		return nil, err
	}

	// Create URL query parameters
	params := url.Values{}
	params.Add("optimize_streaming_latency", strconv.Itoa(e.OptimizeStreamingLatency))
	baseUrl.RawQuery = params.Encode()

	// Create a new HTTP request
	req, err := http.NewRequest("POST", baseUrl.String(), bytes.NewBuffer(jsonReq))
	if err != nil {
		return nil, err
	}

	// Set the headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("accept", "audio/mpeg")

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Check for a successful status code
	if resp.StatusCode != http.StatusOK {
		return nil, err
	}

	// Return the response body (an io.ReadCloser) to the caller
	return resp.Body, nil
}
