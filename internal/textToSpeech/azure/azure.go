package azure

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
)

const (
	subscriptionKey = "3b517889eef943d189356e4a7b0bae35"
	region          = "eastus"
	tokenEndpoint   = "https://" + region + ".api.cognitive.microsoft.com/sts/v1.0/issueToken"
	ttsEndpoint     = "https://" + region + ".tts.speech.microsoft.com/cognitiveservices/v1"
)

type SSML struct {
	XMLName xml.Name `xml:"speak"`
	Version string   `xml:"version,attr"`
	Lang    string   `xml:"xml:lang,attr"`
	Voice   Voice    `xml:"voice"`
}

type Voice struct {
	XMLName xml.Name `xml:"voice"`
	Lang    string   `xml:"xml:lang,attr"`
	Gender  string   `xml:"xml:gender,attr,omitempty"`
	Name    string   `xml:"name,attr"`
	Text    string   `xml:",chardata"`
}

type ReadCloserWrapper struct {
    io.Reader
    Closer func() error
}

func (w *ReadCloserWrapper) Close() error {
    return w.Closer()
}

func getAccessToken() (string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("POST", tokenEndpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("Ocp-Apim-Subscription-Key", subscriptionKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	token, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

func TextToSpeech(text string) (*ReadCloserWrapper, error) {
	token, err := getAccessToken()
	if err != nil {
		return nil, err
	}

	ssml := SSML{
		Version: "1.0",
		Lang:    "en-US",
		Voice: Voice{
			Lang:   "en-US",
			Name:   "en-US-BrandonNeural",
			Gender: "Male",
			Text:   text,
		},
	}

	ssmlBytes, err := xml.Marshal(ssml)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	req, err := http.NewRequest("POST", ttsEndpoint, bytes.NewReader(ssmlBytes))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Bearer "+token)
	req.Header.Add("Content-Type", "application/ssml+xml")
	req.Header.Add("User-Agent", "AzureTextToSpeech")
	req.Header.Add("X-Microsoft-OutputFormat", "ogg-48khz-16bit-mono-opus")

	resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    return &ReadCloserWrapper{
        Reader: resp.Body,
        Closer: func() error {
            return resp.Body.Close()
        },
    }, nil
}