package calls

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
	"github.com/redis/go-redis/v9"
)

type JoinRequest struct {
	BotID              string `validate:"required"`
	BotToken           string `validate:"required"`
	GuildID            string `validate:"required"`
	ChannelID          string `validate:"required"`
	RedisTranscriptKey string
	Config             Config `validate:"required"`
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

type Call struct {
	startTime           time.Time
	connection          *voice.Conn
	closeSignalChan     chan struct{}
	transcriptSSEChan   chan string
	toolMessagesSSEChan chan responder.SSEMessage
	usageSSEChan        chan string
	responder           *responder.Responder
	transcriber         *speechtotext.Transcriber
}

var callsMutex sync.Mutex
var calls = make(map[string]*Call)

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

		// Create discord client
		discordClient, closeClient, err := discord.NewClient(context.Background(), joinReq.BotToken)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		time.Sleep(time.Second * 1)

		joinCtx := r.Context()
		joinCtx, cancel := context.WithTimeout(joinCtx, time.Second*10)
		defer cancel()

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
		toolMessagesSSEChannel := make(chan responder.SSEMessage)

		// Make sse channel for usage messages
		usageSSEChannel := make(chan string)

		var redisClient redis.Client

		if joinReq.RedisTranscriptKey != "" {
			redisClient = *dependencies.RedisClient
		} else {
			redisClient = redis.Client{}
		}

		ongoingCtx, cancel := context.WithCancel(context.Background())

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
			UsageSSEChannel:        usageSSEChannel,
			RedisClient:            &redisClient,
			RedisTranscriptKey:     joinReq.RedisTranscriptKey,
			TranscriptConfig:       *joinReq.Config.TranscriptConfig,
			BotId:                  discordClient.ID(),
		}

		responder := responder.NewResponder(ongoingCtx, responderArgs)

		transcriber := speechtotext.NewTranscriber(joinReq.Config.BotName, *joinReq.Config.TranscriberConfig, responder)

		Speakers := make(map[snowflake.ID]*discord.Speaker)

		// Create call
		newCall := &Call{
			startTime:           time.Now(),
			connection:          &conn,
			closeSignalChan:     closeSignal,
			transcriptSSEChan:   transcriptSSEChannel,
			toolMessagesSSEChan: toolMessagesSSEChannel,
			usageSSEChan:        usageSSEChannel,
			responder:           responder,
			transcriber:         transcriber,
		}

		callId := joinReq.BotID + "-" + joinReq.GuildID

		// Store the call in the map.
		callsMutex.Lock()
		calls[callId] = newCall
		callsMutex.Unlock()

		go discord.WriteToVoiceConnection(ongoingCtx, &conn, playAudioChannel)

		newSpeakerMutex := sync.Mutex{}
		go discord.HandleIncomingPackets(ongoingCtx, cancel, &discordClient, &conn, Speakers, &newSpeakerMutex, transcriber)

		go func() {
			select {
			case <-closeSignal:
			case <-ongoingCtx.Done():
			}

			responder.Cleanup()

			leaveCtx, leaveCancel := context.WithTimeout(context.Background(), time.Second*10)
			defer leaveCancel()
			conn.Close(leaveCtx)
			closeClient()

			// Clean up the call from the calls map.
			callsMutex.Lock()
			delete(calls, callId)
			callsMutex.Unlock()

			// Cancel the context.
			cancel()

			// Close the closeSignal channel.
			close(closeSignal)
		}()

		w.Write([]byte("Joined voice channel"))
	}
}

func LeaveVoiceChannel(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		callId := chi.URLParam(r, "bot_id") + "-" + chi.URLParam(r, "guild_id")

		callsMutex.Lock()
		defer callsMutex.Unlock()

		call, ok := calls[callId]
		if !ok {
			w.Write([]byte("Not in voice call"))
			return
		}

		// Send a signal on the closeSignal channel.
		call.closeSignalChan <- struct{}{}

		// Remove the closeSignal from the closeChannels map.
		delete(calls, callId)

		w.Write([]byte("Left voice call"))
	})
}

func UpdateConfig(dependencies *deps.Deps) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		var config Config

		// Create a new validator instance
		validate := dependencies.Validate

		callId := chi.URLParam(r, "bot_id") + "-" + chi.URLParam(r, "guild_id")

		// Get the call for the given guildID
		call, ok := calls[callId]
		if !ok {
			http.Error(w, "Call not found", http.StatusNotFound)
			return
		}

		err := helpers.DecodeJSONBody(w, r, &config)
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

		if config.TranscriberConfig != nil {
			if err := validate.Struct(config.TranscriberConfig); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			call.transcriber.Config = *config.TranscriberConfig

		}

		if config.VoiceUXConfig != nil {
			if err := validate.Struct(config.VoiceUXConfig); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			call.responder.VoiceUXConfig = *config.VoiceUXConfig
		}

		if config.PromptContents != nil {
			if err := validate.Struct(config.PromptContents); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// oldNumDocuments := len(call.responder.PromptContents.Documents)
			// newNumDocuments := len(config.PromptContents.Documents)

			oldNumTasks := len(call.responder.PromptContents.Tasks)
			newNumTasks := len(config.PromptContents.Tasks)

			shouldRespond := oldNumTasks < newNumTasks

			call.responder.PromptContents = *config.PromptContents

			if shouldRespond && time.Since(call.startTime) > time.Second*3 {
				// Get names of the new documents
				// newDocuments := call.responder.PromptContents.Documents[oldNumDocuments:]
				// newDocumentNames := make([]string, len(newDocuments))
				// for i, doc := range newDocuments {
				// 	newDocumentNames[i] = doc.Name
				// }

				// Get names of the new tasks
				newTasks := call.responder.PromptContents.Tasks[oldNumTasks:]
				newTaskNames := make([]string, len(newTasks))
				for i, task := range newTasks {
					newTaskNames[i] = task.Name
				}

				call.responder.Transcript.AddTaskReminderLine(newTaskNames[0])
				call.responder.AttemptToRespond(false)
			}
		}

		if config.TranscriptConfig != nil {
			if err := validate.Struct(config.TranscriptConfig); err != nil {
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
		callId := chi.URLParam(r, "bot_id") + "-" + chi.URLParam(r, "guild_id")

		callsMutex.Lock()
		call, ok := calls[callId]
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
			case transcriptLine, ok := <-sseChannelForGuild:
				if !ok {
					// The channel has been closed.
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", transcriptLine)
				flusher.Flush()
			}
		}
	})
}

func ToolMessagesSSEHandler(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callId := chi.URLParam(r, "bot_id") + "-" + chi.URLParam(r, "guild_id")

		callsMutex.Lock()
		call, ok := calls[callId]
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
			case toolMessage, ok := <-toolMessagesSSEChannel:
				if !ok {
					// The channel has been closed.
					return
				}

				// Marshal toolMessage to JSON
				jsonToolMessage, err := json.Marshal(toolMessage)
				if err != nil {
					fmt.Println(err)
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", string(jsonToolMessage))
				flusher.Flush()
			}
		}
	})
}

func UsageSSEHandler(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callId := chi.URLParam(r, "bot_id") + "-" + chi.URLParam(r, "guild_id")

		callsMutex.Lock()
		call, ok := calls[callId]
		if !ok {
			http.Error(w, "Not in voice call", http.StatusNotFound)
			return
		}
		usageSSEChannel := call.usageSSEChan
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

		// Listen for new usage updates and send them to the client
		for {
			select {
			case <-r.Context().Done():
				return
			case usageUpdate, ok := <-usageSSEChannel:
				if !ok {
					// The channel has been closed.
					return
				}
				fmt.Fprintf(w, "data: %s\n\n", usageUpdate)
				flusher.Flush()
			}
		}
	})
}
