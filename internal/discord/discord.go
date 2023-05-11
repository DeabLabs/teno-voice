package discord

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"com.deablabs.teno-voice/internal/responder"
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
	responder           *responder.Responder
}

func (s *Speaker) Init(ctx context.Context, responder *responder.Responder) {
	newContext, cancel := context.WithCancel(context.Background())
	s.StreamContext = newContext
	s.ContextCancel = cancel
	s.responder = responder

	wsc, err := speechtotext.NewStream(s.StreamContext, s.Close, responder, s.Username, s.ID.String())

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

	s.transcriptionStream.WriteMessage(websocket.BinaryMessage, packet)
}

func SetupVoiceConnection(ctx context.Context, clientAdress *bot.Client, guildID, channelID snowflake.ID) (voice.Conn, error) {
	client := *clientAdress
	conn := client.VoiceManager().CreateConn(guildID)

	ctx, cancel := context.WithTimeout(ctx, time.Second*5)
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

func HandleIncomingPackets(ctx context.Context, clientAdress *bot.Client, connection *voice.Conn, speakers map[snowflake.ID]*Speaker, newSpeakerMutex *sync.Mutex, responder *responder.Responder) {
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
			if responder.IsIgnored(userID.String()) {
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

				s.Init(ctx, responder)
			}
			newSpeakerMutex.Unlock()

			// add the packet to the speaker
			speakers[userID].AddPacket(ctx, packet.Opus)
		}
	}
}
