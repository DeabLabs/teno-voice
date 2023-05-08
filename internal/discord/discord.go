package discord

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"com.deablabs.teno-voice/internal/deps"
	"com.deablabs.teno-voice/internal/responder"
	speechtotext "com.deablabs.teno-voice/internal/speechToText"
	"com.deablabs.teno-voice/internal/textToSpeech/azure"
	"com.deablabs.teno-voice/pkg/helpers"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/go-chi/chi"
	"github.com/gorilla/websocket"
)

type JoinRequest struct {
	GuildID   string
	ChannelID string
}

type LeaveRequest struct {
	GuildID string
}

type CallStatus struct {
	IsInCall bool
	Err      error
}

type Speaker struct {
	ID                  snowflake.ID
	transcriptionStream *websocket.Conn
	Mu                  sync.Mutex
	StreamContext       context.Context
	ContextCancel       context.CancelFunc
	StreamActive        bool
	responder           *responder.Responder
}

var connectionsMutex sync.Mutex
var connections = make(map[snowflake.ID]voice.Conn)
var transcriptSSEChannels = make(map[snowflake.ID]chan string)

func (s *Speaker) Init(ctx context.Context, responder *responder.Responder) {
	newContext, cancel := context.WithCancel(context.Background())
	s.StreamContext = newContext
	s.ContextCancel = cancel
	s.responder = responder

	wsc, err := speechtotext.NewStream(s.StreamContext, s.Close, responder, s.ID)

	if err != nil {
		panic("error getting transcription stream: " + err.Error())
	}

	s.transcriptionStream = wsc
	s.transcriptionStream.WriteMessage(websocket.BinaryMessage, voice.SilenceAudioFrame)
	s.StreamActive = true
}

func (s *Speaker) Close() {
	s.transcriptionStream.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	s.ContextCancel()
	s.transcriptionStream.Close()
	s.StreamActive = false
}

func (s *Speaker) AddPacket(ctx context.Context, packet []byte) {
	s.Mu.Lock()
	defer s.Mu.Unlock()

	if !s.StreamActive {
		s.Init(ctx, s.responder)
	}

	// convert the opus packet to pcm ogg
	s.transcriptionStream.WriteMessage(websocket.BinaryMessage, packet)
}

func decodeAndValidateRequest(w http.ResponseWriter, r *http.Request) (snowflake.ID, snowflake.ID, error) {
	var jr JoinRequest

	err := helpers.DecodeJSONBody(w, r, &jr)
	if err != nil {
		var mr *helpers.MalformedRequest
		if errors.As(err, &mr) {
			return 0, 0, fmt.Errorf(mr.Msg)
		} else {
			return 0, 0, fmt.Errorf(http.StatusText(http.StatusInternalServerError)+": %s", err.Error())
		}
	}

	guildID, err := snowflake.Parse(jr.GuildID)
	if err != nil {
		return 0, 0, fmt.Errorf("error parsing guildID: %s", err.Error())
	}

	channelID, err := snowflake.Parse(jr.ChannelID)
	if err != nil {
		return 0, 0, fmt.Errorf("error parsing channelID: %s", err.Error())
	}

	return guildID, channelID, nil
}

func setupVoiceConnection(ctx context.Context, clientAdress *bot.Client, guildID, channelID snowflake.ID) (voice.Conn, error) {
	client := *clientAdress
	conn := client.VoiceManager().CreateConn(guildID)

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
	defer cancel()
	if err := conn.Open(ctx, channelID, false, false); err != nil {
		return nil, fmt.Errorf("error connecting to voice channel: %s", err.Error())
	}

	return conn, nil
}

func writeToVoiceConnection(connection *voice.Conn, playAudioChannel chan []byte) {
	conn := *connection
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		select {
		case audioBytes, ok := <-playAudioChannel:
			if !ok {
				return
			}
			if _, err := conn.UDP().Write(audioBytes); err != nil {
				fmt.Printf("error sending audio bytes: %s\n", err)
			}
			// time.Sleep(20 * time.Millisecond) // Add a short sleep
		default:
		}
	}
}

func handleIncomingPackets(ctx context.Context, clientAdress *bot.Client, connection *voice.Conn, speakers map[snowflake.ID]*Speaker, newSpeakerMutex *sync.Mutex, responder *responder.Responder) {
	conn := *connection
	client := *clientAdress

	for {
		packet, err := conn.UDP().ReadPacket()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				println("connection closed")
				return
			}
			fmt.Printf("error while reading from reader: %s", err)
			continue
		}

		userID := conn.UserIDBySSRC(packet.SSRC)

		// ignore packets from the bot user itself
		if userID == client.ID() {
			continue
		}

		// create a speaker for the user if one doesn't exist
		newSpeakerMutex.Lock()
		if _, ok := speakers[userID]; !ok {
			s := &Speaker{
				ID: userID,
				Mu: sync.Mutex{},
			}

			speakers[userID] = s

			s.Init(ctx, responder)
		}
		newSpeakerMutex.Unlock()

		// add the packet to the speaker
		speakers[userID].AddPacket(ctx, packet.Opus)
	}
}

func JoinVoiceCall(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		guildID, channelID, err := decodeAndValidateRequest(w, r)

		if err != nil {
			w.Write([]byte(fmt.Sprintf("Could not join voice call: %s", err.Error())))
			return
		}

		client := *dependencies.DiscordClient
		conn, err := setupVoiceConnection(ctx, &client, guildID, channelID)

		if err != nil {
			w.Write([]byte(fmt.Sprintf("Could not join voice call: %s", err.Error())))
			return
		}

		// Store the connection in the connections map.
		connectionsMutex.Lock()
		connections[guildID] = conn
		connectionsMutex.Unlock()

		if err := conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone); err != nil {
			panic("error setting speaking flag: " + err.Error())
		}

		if _, err := conn.UDP().Write(voice.SilenceAudioFrame); err != nil {
			panic("error sending silence: " + err.Error())
		}

		Speakers := make(map[snowflake.ID]*Speaker)
		playAudioChannel := make(chan []byte)
		azureTTS := azure.NewAzureTTS()
		// Create a responderConfig
		responderConfig := responder.ResponderConfig{
			BotName:                    "Teno",
			SleepMode:                  responder.AutoSleep,
			LinesBeforeSleep:           5,
			BotNameConfidenceThreshold: 0.7,
			LLMService:                 "openai",
			LLMModel:                   "gpt-3.5-turbo",
			TranscriptContextSize:      20,
		}

		// Make sse channel and store it in the map
		transcriptSSEChannel := make(chan string)
		connectionsMutex.Lock()
		transcriptSSEChannels[guildID] = transcriptSSEChannel
		connectionsMutex.Unlock()

		responder := responder.NewResponder(playAudioChannel, azureTTS, responderConfig, transcriptSSEChannel)

		go writeToVoiceConnection(&conn, playAudioChannel)

		newSpeakerMutex := sync.Mutex{}
		go handleIncomingPackets(ctx, &client, &conn, Speakers, &newSpeakerMutex, responder)

		// Create a channel to wait for a signal to close the connection.
		closeSignal := make(chan struct{})
		go func() {
			<-closeSignal
			ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel2()
			conn.Close(ctx2)
		}()

		w.Write([]byte("Joined voice call"))
	})
}

func LeaveVoiceCall(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := *dependencies.DiscordClient

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

		conn := client.VoiceManager().CreateConn(guildID)

		if conn.ChannelID() == nil {
			w.Write([]byte("Not in voice call"))
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		conn.Close(ctx)

		connectionsMutex.Lock()
		conn, ok := connections[guildID]
		if !ok {
			connectionsMutex.Unlock()
			w.Write([]byte("Not in voice call"))
			return
		}

		// Remove the connection from the connections map.
		delete(connections, guildID)
		connectionsMutex.Unlock()

		ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel2()
		conn.Close(ctx2)

		// close the connection.
		conn.Close(ctx)

		w.Write([]byte("Left voice call"))
	})
}

func TranscriptSSEHandler(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		guildIDStr := chi.URLParam(r, "guild_id")

		guildID, err := snowflake.Parse(guildIDStr)
		if err != nil {
			http.Error(w, "Invalid guild ID", http.StatusBadRequest)
			return
		}

		connectionsMutex.Lock()
		sseChannelForGuild, ok := transcriptSSEChannels[guildID]
		connectionsMutex.Unlock()

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
