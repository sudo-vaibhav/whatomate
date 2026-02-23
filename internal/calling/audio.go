package calling

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/pion/rtp"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media/oggreader"
)

// AudioPlayer handles playing pre-recorded OGG/Opus audio into a WebRTC track.
type AudioPlayer struct {
	track *webrtc.TrackLocalStaticRTP
	stop  chan struct{}
}

// NewAudioPlayer creates a new audio player for a WebRTC track.
func NewAudioPlayer(track *webrtc.TrackLocalStaticRTP) *AudioPlayer {
	return &AudioPlayer{
		track: track,
		stop:  make(chan struct{}),
	}
}

// PlayFile plays an OGG/Opus audio file into the WebRTC track.
// It parses the OGG container and sends each Opus packet as a properly
// constructed RTP packet with correct timestamps.
func (p *AudioPlayer) PlayFile(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	ogg, _, err := oggreader.NewWith(file)
	if err != nil {
		return fmt.Errorf("failed to create OGG reader: %w", err)
	}

	// Opus at 48kHz, 20ms frames = 960 samples per frame
	const samplesPerFrame = 960

	var sequenceNumber uint16
	var timestamp uint32

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-p.stop:
			return nil
		case <-ticker.C:
			pageData, _, err := ogg.ParseNextPage()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return fmt.Errorf("failed to parse OGG page: %w", err)
			}

			// Skip OGG header pages (OpusHead and OpusTags)
			if isOpusHeader(pageData) {
				continue
			}

			// Send as a proper RTP packet
			packet := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    111, // Opus
					SequenceNumber: sequenceNumber,
					Timestamp:      timestamp,
					SSRC:           1,
				},
				Payload: pageData,
			}

			if err := p.track.WriteRTP(packet); err != nil {
				return fmt.Errorf("failed to write RTP packet: %w", err)
			}

			sequenceNumber++
			timestamp += samplesPerFrame
		}
	}
}

// Stop stops the current audio playback
func (p *AudioPlayer) Stop() {
	select {
	case <-p.stop:
		// Already stopped/closed
	default:
		close(p.stop)
	}
}

// IsStopped returns true if the player has been stopped.
func (p *AudioPlayer) IsStopped() bool {
	select {
	case <-p.stop:
		return true
	default:
		return false
	}
}

// PlayFileLoop plays an OGG/Opus audio file in a continuous loop until Stop() is called.
func (p *AudioPlayer) PlayFileLoop(filePath string) error {
	for {
		if err := p.PlayFile(filePath); err != nil {
			return err
		}
		// Check stop between loop iterations
		select {
		case <-p.stop:
			return nil
		default:
		}
	}
}

// PlaySilence sends silence packets for the specified duration.
// This keeps the RTP stream alive during pauses.
func (p *AudioPlayer) PlaySilence(duration time.Duration) {
	// Opus silence frame (a minimal valid Opus packet representing silence)
	silence := []byte{0xF8, 0xFF, 0xFE}

	const samplesPerFrame = 960
	var sequenceNumber uint16
	var timestamp uint32

	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.After(duration)
	for {
		select {
		case <-p.stop:
			return
		case <-deadline:
			return
		case <-ticker.C:
			packet := &rtp.Packet{
				Header: rtp.Header{
					Version:        2,
					PayloadType:    111,
					SequenceNumber: sequenceNumber,
					Timestamp:      timestamp,
					SSRC:           1,
				},
				Payload: silence,
			}
			if err := p.track.WriteRTP(packet); err != nil {
				return
			}
			sequenceNumber++
			timestamp += samplesPerFrame
		}
	}
}

// isOpusHeader returns true if the payload is an OpusHead or OpusTags header page.
func isOpusHeader(payload []byte) bool {
	if len(payload) < 8 {
		return false
	}
	header := string(payload[:8])
	return header == "OpusHead" || header == "OpusTags"
}
