package azure

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"

	Config "com.deablabs.teno-voice/internal/config"
	"mccoy.space/g/ogg"
)

const (
	region        = "eastus"
	tokenEndpoint = "https://" + region + ".api.cognitive.microsoft.com/sts/v1.0/issueToken"
	ttsEndpoint   = "https://" + region + ".tts.speech.microsoft.com/cognitiveservices/v1"
)

var subscriptionKey = Config.Environment.AzureToken

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

type AzureTTS struct{}

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

func (a *AzureTTS) Synthesize(text string) (<-chan []byte, error) {
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

	opusPackets, err := oggToOpusPackets(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	// Close the audio stream when done reading Opus packets
	go func() {
		for range opusPackets {
		}
		resp.Body.Close()
	}()

	return opusPackets, nil
}

func oggToOpusPackets(reader io.Reader) (<-chan []byte, error) {
	decoder := ogg.NewDecoder(reader)
	opusPackets := make(chan []byte)

	go func() {
		defer close(opusPackets)
		for {
			page, err := decoder.Decode()
			if err != nil {
				if err == io.EOF {
					break
				}
				fmt.Printf("Error decoding Ogg page: %s\n", err)
				return
			}

			for _, packet := range page.Packets {
				opusPackets <- packet
			}
		}
	}()

	return opusPackets, nil
}
