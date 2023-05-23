package calls

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"sync"
	"time"

	"com.deablabs.teno-voice/internal/deps"
	"com.deablabs.teno-voice/internal/discord"
	"com.deablabs.teno-voice/internal/llm"
	"com.deablabs.teno-voice/internal/llm/promptbuilder"
	"com.deablabs.teno-voice/internal/responder"
	speechtotext "com.deablabs.teno-voice/internal/speechToText"
	texttospeech "com.deablabs.teno-voice/internal/textToSpeech"
	"com.deablabs.teno-voice/internal/transcript"
	"com.deablabs.teno-voice/pkg/helpers"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/go-chi/chi"
)

type JoinRequest struct {
	GuildID            string
	ChannelID          string
	RedisTranscriptKey string
	Config             Config
}

type ValidatedJoinRequest struct {
	GuildID            snowflake.ID
	ChannelID          snowflake.ID
	RedisTranscriptKey string
	Config             Config
}

type Config struct {
	BotName           string                          `validate:"required"`
	PromptContents    *promptbuilder.PromptContents   `validate:"required"`
	VoiceUXConfig     *responder.VoiceUXConfig        `validate:"required"`
	LLMConfig         *llm.LLMConfigPayload           `validate:"required,LLMConfigValidation"`
	TTSConfig         *texttospeech.TTSConfigPayload  `validate:"required,TTSConfigValidation"`
	TranscriptConfig  *transcript.TranscriptConfig    `validate:"required"`
	TranscriberConfig *speechtotext.TranscriberConfig `validate:"required"`
}

type LeaveRequest struct {
	GuildID string
}

type Call struct {
	connection          *voice.Conn
	closeSignalChan     chan struct{}
	transcriptSSEChan   chan string
	toolMessagesSSEChan chan string
	responder           *responder.Responder
	transcriber         *speechtotext.Transcriber
}

var callsMutex sync.Mutex
var calls = make(map[snowflake.ID]*Call)

func JoinVoiceChannel(dependencies *deps.Deps) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var joinReq JoinRequest

		err := helpers.DecodeJSONBody(w, r, &joinReq)
		if err != nil {
			var mr *helpers.MalformedRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.Msg, mr.Status)
			} else {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		joinCtx := r.Context()
		joinCtx, cancel := context.WithTimeout(joinCtx, time.Second*5)
		defer cancel()

		// Validate Snowflake IDs
		guildID, err := snowflake.Parse(joinReq.GuildID)
		if err != nil {
			http.Error(w, "Invalid Guild ID", http.StatusBadRequest)
			return
		}

		channelID, err := snowflake.Parse(joinReq.ChannelID)
		if err != nil {
			http.Error(w, "Invalid Channel ID", http.StatusBadRequest)
			return
		}

		// Create a new validator instance
		validate := dependencies.Validate

		// Validate the struct
		if err := validate.Struct(&joinReq); err != nil {
			// Return an error to the client if the struct is not valid
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Create tts service
		tts, err := texttospeech.ParseTTSConfig(*joinReq.Config.TTSConfig)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Create llm service
		llm, err := llm.ParseLLMConfig(*joinReq.Config.LLMConfig)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		discordClient := *dependencies.DiscordClient

		// Setup voice connection
		conn, err := discord.SetupVoiceConnection(joinCtx, &discordClient, guildID, channelID)

		if err != nil {
			w.Write([]byte(fmt.Sprintf("Could not join voice call: %s", err.Error())))
			return
		}

		if err := conn.SetSpeaking(joinCtx, voice.SpeakingFlagMicrophone); err != nil {
			panic("error setting speaking flag: " + err.Error())
		}

		if _, err := conn.UDP().Write(voice.SilenceAudioFrame); err != nil {
			panic("error sending silence: " + err.Error())
		}

		// Create a channel to wait for a signal to close the connection.
		closeSignal := make(chan struct{})

		// Make sse channel for live transcript updates
		transcriptSSEChannel := make(chan string)

		// Make sse channel for tool messages
		toolMessagesSSEChannel := make(chan string)

		redisClient := *dependencies.RedisClient

		playAudioChannel := make(chan []byte)

		responderArgs := responder.NewResponderArgs{
			BotName:                joinReq.Config.BotName,
			PlayAudioChannel:       playAudioChannel,
			Conn:                   &conn,
			TTSService:             &tts,
			LLMService:             &llm,
			VoiceUXConfig:          *joinReq.Config.VoiceUXConfig,
			PromptContents:         joinReq.Config.PromptContents,
			TranscriptSSEChannel:   transcriptSSEChannel,
			ToolMessagesSSEChannel: toolMessagesSSEChannel,
			RedisClient:            &redisClient,
			RedisTranscriptKey:     joinReq.RedisTranscriptKey,
			TranscriptConfig:       *joinReq.Config.TranscriptConfig,
			BotId:                  discordClient.ID(),
		}

		responder := responder.NewResponder(responderArgs)

		transcriber := speechtotext.NewTranscriber(joinReq.Config.BotName, *joinReq.Config.TranscriberConfig, responder)

		Speakers := make(map[snowflake.ID]*discord.Speaker)

		// Create call
		newCall := &Call{
			connection:          &conn,
			closeSignalChan:     closeSignal,
			transcriptSSEChan:   transcriptSSEChannel,
			toolMessagesSSEChan: toolMessagesSSEChannel,
			responder:           responder,
			transcriber:         transcriber,
		}

		// Store the call in the map.
		callsMutex.Lock()
		calls[guildID] = newCall
		callsMutex.Unlock()

		ongoingCtx, cancel := context.WithCancel(context.Background())

		go discord.WriteToVoiceConnection(ongoingCtx, &conn, playAudioChannel)

		newSpeakerMutex := sync.Mutex{}
		go discord.HandleIncomingPackets(ongoingCtx, &discordClient, &conn, Speakers, &newSpeakerMutex, transcriber)

		go func() {
			<-closeSignal
			leaveCtx, leaveCancel := context.WithTimeout(context.Background(), time.Second*10)
			defer leaveCancel()
			conn.Close(leaveCtx)

			// Clean up the call from the calls map.
			callsMutex.Lock()
			delete(calls, guildID)
			callsMutex.Unlock()

			// Cancel the context.
			cancel()
		}()

		w.Write([]byte("Joined voice channel"))
	}
}

func LeaveVoiceChannel(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("LeaveVoiceCall called")

		var lr LeaveRequest

		err := helpers.DecodeJSONBody(w, r, &lr)
		if err != nil {
			var mr *helpers.MalformedRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.Msg, mr.Status)
			} else {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		guildID, err := snowflake.Parse(lr.GuildID)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		callsMutex.Lock()
		defer callsMutex.Unlock()

		call, ok := calls[guildID]
		if !ok {
			w.Write([]byte("Not in voice call"))
			return
		}

		// Send a signal on the closeSignal channel.
		call.closeSignalChan <- struct{}{}

		// Remove the closeSignal from the closeChannels map.
		delete(calls, guildID)

		w.Write([]byte("Left voice call"))
	})
}

func UpdateConfig(dependencies *deps.Deps) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var config Config

		// Create a new validator instance
		validate := dependencies.Validate

		// Validate the struct

		guildIDStr := chi.URLParam(r, "guild_id")

		guildID, err := snowflake.Parse(guildIDStr)
		if err != nil {
			http.Error(w, "Invalid guild ID", http.StatusBadRequest)
			return
		}

		// Get the call for the given guildID
		call, ok := calls[guildID]
		if !ok {
			http.Error(w, "Call not found", http.StatusNotFound)
			return
		}

		err = helpers.DecodeJSONBody(w, r, &config)
		if err != nil {
			var mr *helpers.MalformedRequest
			if errors.As(err, &mr) {
				http.Error(w, mr.Msg, mr.Status)
			} else {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}
			return
		}

		if config.BotName != "" {
			call.transcriber.BotName = config.BotName
			call.responder.BotName = config.BotName
		}

		if config.TranscriberConfig != nil && !reflect.DeepEqual(call.transcriber.Config, config.TranscriberConfig) {
			if err := validate.Struct(&config.TranscriberConfig); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			call.transcriber.Config = *config.TranscriberConfig
		}

		if config.VoiceUXConfig != nil && !reflect.DeepEqual(call.responder.VoiceUXConfig, config.VoiceUXConfig) {
			if err := validate.Struct(&config.VoiceUXConfig); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			call.responder.VoiceUXConfig = *config.VoiceUXConfig
		}

		if config.PromptContents != nil && !reflect.DeepEqual(call.responder.PromptContents, config.PromptContents) {
			if err := validate.Struct(&config.PromptContents); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			call.responder.PromptContents = *config.PromptContents
		}

		if config.TranscriptConfig != nil && !reflect.DeepEqual(call.responder.Transcript.Config, config.TranscriptConfig) {
			if err := validate.Struct(&config.TranscriptConfig); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			call.responder.Transcript.Config = *config.TranscriptConfig
		}

		if config.TTSConfig != nil {
			tts, err := texttospeech.ParseTTSConfig(*config.TTSConfig)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}
			call.responder.TtsService = tts

		}

		if config.LLMConfig != nil {
			llm, err := llm.ParseLLMConfig(*config.LLMConfig)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
			}

			call.responder.LlmService = llm
		}

		w.WriteHeader(http.StatusOK)
	}
}

func TranscriptSSEHandler(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guildIDStr := chi.URLParam(r, "guild_id")

		guildID, err := snowflake.Parse(guildIDStr)
		if err != nil {
			http.Error(w, "Invalid guild ID", http.StatusBadRequest)
			return
		}

		callsMutex.Lock()
		call, ok := calls[guildID]
		if !ok {
			http.Error(w, "Not in voice call", http.StatusNotFound)
			return
		}
		sseChannelForGuild := call.transcriptSSEChan
		callsMutex.Unlock()

		if !ok {
			http.Error(w, "No active SSE channels for this guild", http.StatusNotFound)
			return
		}

		// Set the necessary headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Use a flusher to send data immediately to the client
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		// Listen for new transcript lines and send them to the client
		for {
			select {
			case <-r.Context().Done():
				return
			case transcriptLine := <-sseChannelForGuild:
				fmt.Fprintf(w, "data: %s\n\n", transcriptLine)
				flusher.Flush()
			}
		}
	})
}

func ToolMessagesSSEHandler(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guildIDStr := chi.URLParam(r, "guild_id")

		guildID, err := snowflake.Parse(guildIDStr)
		if err != nil {
			http.Error(w, "Invalid guild ID", http.StatusBadRequest)
			return
		}

		callsMutex.Lock()
		call, ok := calls[guildID]
		if !ok {
			http.Error(w, "Not in voice call", http.StatusNotFound)
			return
		}
		toolMessagesSSEChannel := call.toolMessagesSSEChan
		callsMutex.Unlock()

		if !ok {
			http.Error(w, "No active SSE channels for this guild", http.StatusNotFound)
			return
		}

		// Set the necessary headers for SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Use a flusher to send data immediately to the client
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
			return
		}

		// Listen for new tool messages and send them to the client
		for {
			select {
			case <-r.Context().Done():
				return
			case toolMessage := <-toolMessagesSSEChannel:
				fmt.Fprintf(w, "data: %s\n\n", toolMessage)
				flusher.Flush()
			}
		}
	})
}
