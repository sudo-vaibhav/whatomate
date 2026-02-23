package calling

import (
	"sync"

	"github.com/pion/webrtc/v4"
)

// AudioBridge forwards RTP packets bidirectionally between two WebRTC tracks.
// It bridges the caller's remote track to the agent's local track, and vice versa.
type AudioBridge struct {
	stop chan struct{}
	wg   sync.WaitGroup
}

// NewAudioBridge creates a new audio bridge.
func NewAudioBridge() *AudioBridge {
	return &AudioBridge{
		stop: make(chan struct{}),
	}
}

// Start begins bidirectional RTP forwarding. It blocks until both directions end.
func (b *AudioBridge) Start(
	callerRemote *webrtc.TrackRemote, agentLocal *webrtc.TrackLocalStaticRTP,
	agentRemote *webrtc.TrackRemote, callerLocal *webrtc.TrackLocalStaticRTP,
) {
	b.wg.Add(2)

	// Caller audio → Agent speaker
	go b.forward(callerRemote, agentLocal)

	// Agent mic → Caller speaker
	go b.forward(agentRemote, callerLocal)

	b.wg.Wait()
}

// forward reads RTP packets from src and writes them to dst until stopped.
func (b *AudioBridge) forward(src *webrtc.TrackRemote, dst *webrtc.TrackLocalStaticRTP) {
	defer b.wg.Done()

	buf := make([]byte, 1500)
	for {
		select {
		case <-b.stop:
			return
		default:
		}

		n, _, err := src.Read(buf)
		if err != nil {
			return
		}

		if _, err := dst.Write(buf[:n]); err != nil {
			return
		}
	}
}

// Stop terminates both forwarding goroutines.
func (b *AudioBridge) Stop() {
	select {
	case <-b.stop:
		// Already stopped
	default:
		close(b.stop)
	}
}
