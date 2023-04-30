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
	"com.deablabs.teno-voice/pkg/helpers"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
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

type OpusPacket struct {
	Bytes     []byte
	Timestamp time.Time
}

type Speaker struct {
	ID      snowflake.ID
	Packets []OpusPacket
	Mu      sync.Mutex
}

func (s *Speaker) AddPacket(packet OpusPacket) {
	s.Mu.Lock()
	s.Packets = append(s.Packets, packet)
	s.Mu.Unlock()

	// after 500 milliseconds, check if the last packet is older than 500 milliseconds
	// if so, log the length of the packets and clear them
	time.AfterFunc(time.Millisecond*500, func() {
		s.Mu.Lock()
		defer s.Mu.Unlock()
		if len(s.Packets) > 0 && time.Since(s.Packets[len(s.Packets)-1].Timestamp) > time.Millisecond*500 {
			fmt.Printf("Speaker %s has %d packets\n", s.ID, len(s.Packets))
			s.Packets = make([]OpusPacket, 0)
		}
	})
}

func JoinVoiceCall(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		ctx, cancel := context.WithTimeout(ctx, time.Second*5)
		defer cancel()

		d := make(chan CallStatus, 1)
		go func() {
			client := *dependencies.DiscordClient

			var jr JoinRequest

			err := helpers.DecodeJSONBody(w, r, &jr)
			if err != nil {
				var mr *helpers.MalformedRequest
				if errors.As(err, &mr) {
					d <- CallStatus{IsInCall: false, Err: fmt.Errorf(mr.Msg)}
				} else {
					d <- CallStatus{IsInCall: false, Err: fmt.Errorf(http.StatusText(http.StatusInternalServerError) + ": " + err.Error())}
				}
				return
			}

			guildID, err := snowflake.Parse(jr.GuildID)
			if err != nil {
				d <- CallStatus{IsInCall: false, Err: fmt.Errorf("error parsing guildID: " + err.Error())}
				return
			}

			channelID, err := snowflake.Parse(jr.ChannelID)
			if err != nil {
				d <- CallStatus{IsInCall: false, Err: fmt.Errorf("error parsing channelID: " + err.Error())}
				return
			}

			conn := client.VoiceManager().CreateConn(guildID)

			ctx, cancel := context.WithTimeout(ctx, time.Second*5)
			defer cancel()
			if err := conn.Open(ctx, channelID, false, false); err != nil {
				d <- CallStatus{IsInCall: false, Err: fmt.Errorf("error connecting to voice channel: " + err.Error())}
				return
			}

			defer func() {
				ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*10)
				defer cancel2()
				conn.Close(ctx2)
			}()

			d <- CallStatus{IsInCall: true, Err: nil}
			println("starting playback")

			if err := conn.SetSpeaking(ctx, voice.SpeakingFlagMicrophone); err != nil {
				panic("error setting speaking flag: " + err.Error())
			}

			if _, err := conn.UDP().Write(voice.SilenceAudioFrame); err != nil {
				panic("error sending silence: " + err.Error())
			}

			Speakers := make(map[snowflake.ID]*Speaker)

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
				if _, ok := Speakers[userID]; !ok {
					Speakers[userID] = &Speaker{
						ID:      userID,
						Packets: make([]OpusPacket, 0),
						Mu:      sync.Mutex{},
					}
				}

				// add the packet to the speaker
				Speakers[userID].AddPacket(OpusPacket{
					Bytes:     packet.Opus,
					Timestamp: time.Now(),
				})
			}
		}()

		select {
		case <-ctx.Done():
			w.Write([]byte("Timeout joining voice call"))
			return

		case result := <-d:
			if result.Err != nil {
				w.Write([]byte("Could not join voice call: " + result.Err.Error()))
				return
			}

			if result.IsInCall {
				w.Write([]byte("Joined voice call"))
			} else {
				w.Write([]byte("Could not join voice call"))
			}
			return
		}
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

		w.Write([]byte("Left voice call"))
	})
}
