package discord

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	speechtotext "com.deablabs.teno-voice/internal/speechToText"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
	"github.com/gorilla/websocket"
)

type Speaker struct {
	ID                  snowflake.ID
	Username            string
	transcriptionStream *websocket.Conn
	Mu                  sync.Mutex
	StreamContext       context.Context
	ContextCancel       context.CancelFunc
	StreamActive        bool
	transcriber         *speechtotext.Transcriber
}

func (s *Speaker) Init(ctx context.Context, transcriber *speechtotext.Transcriber) {
	newContext, cancel := context.WithCancel(context.Background())
	s.StreamContext = newContext
	s.ContextCancel = cancel
	s.transcriber = transcriber

	wsc, err := s.transcriber.NewStream(s.StreamContext, s.Close, s.Username, s.ID.String())

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
		s.Init(ctx, s.transcriber)
	}

	s.transcriptionStream.WriteMessage(websocket.BinaryMessage, packet)
}

func SetupVoiceConnection(ctx context.Context, clientAdress *bot.Client, guildID, channelID snowflake.ID) (voice.Conn, error) {
	client := *clientAdress
	conn := client.VoiceManager().CreateConn(guildID)

	ctx, cancel := context.WithTimeout(ctx, time.Second*10)
	defer cancel()
	if err := conn.Open(ctx, channelID, false, false); err != nil {
		return nil, fmt.Errorf("error connecting to voice channel: %s", err.Error())
	}

	return conn, nil
}

func WriteToVoiceConnection(ctx context.Context, connection *voice.Conn, playAudioChannel chan []byte) {
	conn := *connection

	lastFrameSent := time.Now()

	for {
		select {
		case audioBytes, ok := <-playAudioChannel:
			if !ok {
				return
			}

			// Write audio bytes to UDP connection
			if _, err := conn.UDP().Write(audioBytes); err != nil {
				fmt.Printf("error sending audio bytes: %s\n", err)
			}

			// Calculate sleep time
			sleepTime := 20*time.Millisecond - time.Since(lastFrameSent)
			if sleepTime > 0 {
				time.Sleep(sleepTime)
			}

			// Update the lastFrameSent timestamp
			lastFrameSent = time.Now()

		case <-ctx.Done():
			return
		}
	}
}

func HandleIncomingPackets(ctx context.Context, cancelFunc context.CancelFunc, clientAdress *bot.Client, connection *voice.Conn, speakers map[snowflake.ID]*Speaker, newSpeakerMutex *sync.Mutex, transcriber *speechtotext.Transcriber) {
	conn := *connection
	client := *clientAdress

	for {
		select {
		case <-ctx.Done():
			return
		default:
			packet, err := conn.UDP().ReadPacket()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					println("connection closed")
					cancelFunc()
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

			// ignore packets from the responder's ignore list
			if transcriber.IsIgnored(userID.String()) {
				continue
			}

			// create a speaker for the user if one doesn't exist
			newSpeakerMutex.Lock()
			if _, ok := speakers[userID]; !ok {
				var username string
				user, err := client.Rest().GetMember(conn.GuildID(), userID)
				if err != nil {
					fmt.Printf("error getting user: %s", err)
					username = "User"
				} else {
					username = user.User.Username
				}

				s := &Speaker{
					ID:       userID,
					Mu:       sync.Mutex{},
					Username: username,
				}

				speakers[userID] = s

				s.Init(ctx, transcriber)
			}
			newSpeakerMutex.Unlock()

			// add the packet to the speaker
			speakers[userID].AddPacket(ctx, packet.Opus)
		}
	}
}
