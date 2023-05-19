package azure

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	Config "com.deablabs.teno-voice/internal/config"
	"com.deablabs.teno-voice/internal/usage"
	"mccoy.space/g/ogg"
)

type AzureConfig struct {
	Model    string `validate:"required"`
	VoiceID  string `validate:"required"`
	Language string `validate:"required"`
	Gender   string `validate:"required"`
}

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

type AzureTTS struct {
	Config AzureConfig
}

func NewAzureTTS(config AzureConfig) *AzureTTS {
	return &AzureTTS{
		Config: config,
	}
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

func (a *AzureTTS) Synthesize(text string) (io.ReadCloser, error) {
	token, err := getAccessToken()
	if err != nil {
		return nil, err
	}

	ssml := SSML{
		Version: "1.0",
		Lang:    a.Config.Language,
		Voice: Voice{
			Lang:   a.Config.Language,
			Name:   a.Config.VoiceID,
			Gender: a.Config.Gender,
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

	opusReader := NewOpusPacketReader(resp.Body)

	usage.NewTextToSpeechEvent("azure", a.Config.Model, len(text))

	return opusReader, nil
}

type OpusPacketReader struct {
	reader      io.ReadCloser
	oggDecoder  *ogg.Decoder
	packetChan  chan []byte
	errChan     chan error
	closeSignal chan struct{}
	lastRead    time.Time
}

func NewOpusPacketReader(reader io.ReadCloser) *OpusPacketReader {
	opr := &OpusPacketReader{
		reader:      reader,
		oggDecoder:  ogg.NewDecoder(reader),
		packetChan:  make(chan []byte),
		errChan:     make(chan error),
		closeSignal: make(chan struct{}),
		lastRead:    time.Now(),
	}

	go opr.processOggStream()

	return opr
}

func (o *OpusPacketReader) processOggStream() {
	defer close(o.packetChan)
	defer close(o.errChan)

	for {
		select {
		case <-o.closeSignal:
			return
		default:
			page, err := o.oggDecoder.Decode()
			if err != nil {
				o.errChan <- err
				return
			}

			for _, packet := range page.Packets {
				o.packetChan <- packet
				sleepTime := 20*time.Millisecond - time.Since(o.lastRead)
				if sleepTime > 0 {
					time.Sleep(sleepTime)
				}
				o.lastRead = time.Now()
			}
		}
	}
}

func (o *OpusPacketReader) Read(p []byte) (int, error) {
	select {
	case packet, ok := <-o.packetChan:
		if !ok {
			return 0, io.EOF
		}
		n := copy(p, packet)
		return n, nil
	case err := <-o.errChan:
		return 0, err
	}
}

func (o *OpusPacketReader) Close() error {
	close(o.closeSignal)
	return o.reader.Close()
}
