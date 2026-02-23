package calling

import (
	"context"
	"fmt"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
)

// negotiateWebRTC handles the SDP offer/answer exchange and sets up WebRTC media
func (m *Manager) negotiateWebRTC(session *CallSession, account *models.WhatsAppAccount, sdpOffer string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	// Pre-accept the call to keep it alive while we set up WebRTC
	if err := m.whatsapp.PreAcceptCall(ctx, waAccount, session.ID); err != nil {
		m.log.Error("Failed to pre-accept call", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	// Create peer connection with Opus codec
	pc, err := m.createPeerConnection()
	if err != nil {
		m.log.Error("Failed to create peer connection", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	session.mu.Lock()
	session.PeerConnection = pc
	session.mu.Unlock()

	// Add local audio track for IVR playback
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio",
		"ivr-audio",
	)
	if err != nil {
		m.log.Error("Failed to create audio track", "error", err)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	_, err = pc.AddTrack(audioTrack)
	if err != nil {
		m.log.Error("Failed to add audio track", "error", err)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	session.mu.Lock()
	session.AudioTrack = audioTrack
	session.mu.Unlock()

	// Register handler for incoming audio (caller's voice + DTMF)
	pc.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		m.log.Info("Received remote track",
			"call_id", session.ID,
			"codec", track.Codec().MimeType,
			"payload_type", track.PayloadType(),
		)

		// Check if this is a telephone-event track (DTMF)
		if track.Codec().MimeType == "audio/telephone-event" {
			go m.handleDTMFTrack(session, track)
			return
		}

		// Store the caller's remote track for potential audio bridge use
		session.mu.Lock()
		session.CallerRemoteTrack = track
		session.mu.Unlock()

		// Consume audio to keep the stream flowing; exits when bridge takes over
		go m.consumeAudioTrack(session, track)
	})

	// Handle connection state changes
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		m.log.Info("Peer connection state changed",
			"call_id", session.ID,
			"state", state.String(),
		)
		switch state {
		case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateDisconnected:
			m.EndCall(session.ID)
		}
	})

	// Set the remote SDP offer
	offer := webrtc.SessionDescription{
		Type: webrtc.SDPTypeOffer,
		SDP:  sdpOffer,
	}
	if err := pc.SetRemoteDescription(offer); err != nil {
		m.log.Error("Failed to set remote description", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	// Create SDP answer
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		m.log.Error("Failed to create SDP answer", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	// Set local description
	if err := pc.SetLocalDescription(answer); err != nil {
		m.log.Error("Failed to set local description", "error", err, "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	// Wait for ICE gathering to complete
	gatherComplete := webrtc.GatheringCompletePromise(pc)
	select {
	case <-gatherComplete:
		// ICE gathering complete
	case <-ctx.Done():
		m.log.Error("ICE gathering timed out", "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	// Accept the call with SDP answer
	localDesc := pc.LocalDescription()
	if localDesc == nil {
		m.log.Error("No local description available", "call_id", session.ID)
		m.rejectCall(ctx, waAccount, session.ID)
		return
	}

	if err := m.whatsapp.AcceptCall(ctx, waAccount, session.ID, localDesc.SDP); err != nil {
		m.log.Error("Failed to accept call via API", "error", err, "call_id", session.ID)
		return
	}

	session.mu.Lock()
	session.Status = models.CallStatusAnswered
	session.mu.Unlock()

	m.log.Info("Call accepted with WebRTC", "call_id", session.ID)

	// Start IVR flow if configured
	if session.IVRFlow != nil {
		go m.runIVRFlow(session, waAccount)
	}
}

// createPeerConnection creates a new WebRTC peer connection with Opus codec support
func (m *Manager) createPeerConnection() (*webrtc.PeerConnection, error) {
	// Configure with STUN servers for NAT traversal
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	mediaEngine := &webrtc.MediaEngine{}

	// Register Opus codec
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  webrtc.MimeTypeOpus,
			ClockRate: 48000,
			Channels:  2,
		},
		PayloadType: 111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to register Opus codec: %w", err)
	}

	// Register telephone-event codec for DTMF (RFC 4733)
	if err := mediaEngine.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{
			MimeType:  "audio/telephone-event",
			ClockRate: 8000,
		},
		PayloadType: 101,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		return nil, fmt.Errorf("failed to register telephone-event codec: %w", err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(mediaEngine))
	return api.NewPeerConnection(config)
}

// consumeAudioTrack reads and discards RTP packets to keep the stream active.
// It exits when the bridge takes over (BridgeStarted channel is closed) or on error.
func (m *Manager) consumeAudioTrack(session *CallSession, track *webrtc.TrackRemote) {
	buf := make([]byte, 1500)
	for {
		select {
		case <-session.BridgeStarted:
			// Bridge is taking over reading from this track
			return
		default:
		}

		_, _, err := track.Read(buf)
		if err != nil {
			return
		}
	}
}

// rejectCall sends a reject action via the WhatsApp API
func (m *Manager) rejectCall(ctx context.Context, account *whatsapp.Account, callID string) {
	if err := m.whatsapp.RejectCall(ctx, account, callID); err != nil {
		m.log.Error("Failed to reject call", "error", err, "call_id", callID)
	}
}
