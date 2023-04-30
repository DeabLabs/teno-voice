package discord

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
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
				if _, err = conn.UDP().Write(packet.Opus); err != nil {
					if errors.Is(err, net.ErrClosed) {
						println("connection closed")
						return
					}
					fmt.Printf("error while writing to UDPConn: %s", err)
					continue
				}
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
