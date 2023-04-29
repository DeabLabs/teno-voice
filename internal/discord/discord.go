package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"com.deablabs.teno-voice/internal/deps"
	"github.com/disgoorg/disgo/voice"
	"github.com/disgoorg/snowflake/v2"
)

func JoinVoiceCall(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO Join the voice call
		log.Println("Joining voice call...")
		client := *dependencies.DiscordClient

		guildID, err := snowflake.Parse("795715599405547531")
		if err != nil {
			panic("error parsing guildID: " + err.Error())
		}

		channelID, err := snowflake.Parse("1071504426570883143")
		if err != nil {
			panic("error parsing channelID: " + err.Error())
		}

		conn := client.VoiceManager().CreateConn(guildID)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
		defer cancel()
		if err := conn.Open(ctx, channelID, false, false); err != nil {
			panic("error connecting to voice channel: " + err.Error())
		}

		defer func() {
			ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel2()
			conn.Close(ctx2)
		}()

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
	})
}

func LeaveVoiceCall(dependencies *deps.Deps) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO Leave the voice call
		log.Println("Leaving voice call...")
	})
}
