package calling

import (
	"fmt"
	"os"
	"time"

	"github.com/pion/webrtc/v4"
)

// AudioPlayer handles playing pre-recorded Opus audio into a WebRTC track
type AudioPlayer struct {
	track *webrtc.TrackLocalStaticRTP
	stop  chan struct{}
}

// NewAudioPlayer creates a new audio player for a WebRTC track
func NewAudioPlayer(track *webrtc.TrackLocalStaticRTP) *AudioPlayer {
	return &AudioPlayer{
		track: track,
		stop:  make(chan struct{}),
	}
}

// PlayFile plays an Opus audio file (raw Opus packets, one per line) into the track.
// This is a simplified implementation. In production, you'd parse OGG/Opus containers.
func (p *AudioPlayer) PlayFile(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}

	// For a basic implementation, send the raw audio data as RTP packets
	// In production, this should parse OGG containers and extract Opus frames
	packetSize := 960 // 20ms at 48kHz
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	offset := 0
	for offset < len(data) {
		select {
		case <-p.stop:
			return nil
		case <-ticker.C:
			end := offset + packetSize
			if end > len(data) {
				end = len(data)
			}

			if _, err := p.track.Write(data[offset:end]); err != nil {
				return fmt.Errorf("failed to write audio packet: %w", err)
			}
			offset = end
		}
	}

	return nil
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

// PlayFileLoop plays an audio file in a continuous loop until Stop() is called.
func (p *AudioPlayer) PlayFileLoop(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}

	packetSize := 960 // 20ms at 48kHz
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()

	for {
		offset := 0
		for offset < len(data) {
			select {
			case <-p.stop:
				return nil
			case <-ticker.C:
				end := offset + packetSize
				if end > len(data) {
					end = len(data)
				}
				if _, err := p.track.Write(data[offset:end]); err != nil {
					return fmt.Errorf("failed to write audio packet: %w", err)
				}
				offset = end
			}
		}
		// Check stop between iterations
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
			if _, err := p.track.Write(silence); err != nil {
				return
			}
		}
	}
}
