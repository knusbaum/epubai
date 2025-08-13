package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/ebitengine/oto/v3"
	"github.com/hajimehoshi/go-mp3"
	"github.com/sashabaranov/go-openai"
)

type Speaker struct {
	openaiKey string
	client    *openai.Client
}

func NewSpeaker(openaiApiKey string) Speaker {
	return Speaker{
		openaiKey: openaiApiKey,
	}
}

func (s *Speaker) Speak(ctx context.Context, str string) error {
	if s.client == nil {
		s.client = openai.NewClient(s.openaiKey)
	}
	return oai_speak(ctx, s.client, str)
}

func oai_speak(ctx context.Context, c *openai.Client, s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	resp, err := c.CreateSpeech(
		ctx,
		openai.CreateSpeechRequest{
			Model:          openai.TTSModel1,
			Input:          s,
			Voice:          openai.VoiceAlloy, //openai.VoiceShimmer, //openai.VoiceAlloy,
			ResponseFormat: openai.SpeechResponseFormatMp3,
			Speed:          1.0,
		})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		fmt.Printf("Failed to speak [%s]: %v\n", s, err)
		return err
	}

	defer resp.Close()
	playSound(ctx, resp)
	return nil
}

var (
	octx     *oto.Context
	otoready chan struct{}
)

func playSound(ctx context.Context, r io.ReadCloser) error {
	decodedMp3, err := mp3.NewDecoder(r)
	if err != nil {
		return err
	}
	op := &oto.NewContextOptions{}

	// Usually 44100 or 48000. Other values might cause distortions in Oto
	op.SampleRate = decodedMp3.SampleRate()

	// Number of channels (aka locations) to play sounds from. Either 1 or 2.
	// 1 is mono sound, and 2 is stereo (most speakers are stereo).
	op.ChannelCount = 2

	// Format of the source. go-mp3's format is signed 16bit integers.
	op.Format = oto.FormatSignedInt16LE

	if octx == nil {
		// Remember that you should **not** create more than one context
		octx, otoready, err = oto.NewContext(op)
		if err != nil {
			panic("oto.NewContext failed: " + err.Error())
		}
	}
	// It might take a bit for the hardware audio devices to be ready, so we wait on the channel.
	<-otoready

	// Create a new 'player' that will handle our sound. Paused by default.
	player := octx.NewPlayer(decodedMp3)

	// Play starts playing the sound and returns without waiting for it (Play() is async).
	player.Play()

	// We can wait for the sound to finish playing using something like this
	for player.IsPlaying() {
		time.Sleep(10 * time.Millisecond)
		if ctx.Err() != nil {
			break
		}
	}

	// Now that the sound finished playing, we can restart from the beginning (or go to any location in the sound) using seek
	// newPos, err := player.(io.Seeker).Seek(0, io.SeekStart)
	// if err != nil{
	//     panic("player.Seek failed: " + err.Error())
	// }
	// println("Player is now at position:", newPos)
	// player.Play()

	// If you don't want the player/sound anymore simply close
	err = player.Close()
	if err != nil {
		return fmt.Errorf("player.Close failed: %w", err)
	}
	return nil
}
