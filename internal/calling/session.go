package calling

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pion/webrtc/v4"
	"github.com/shridarpatil/whatomate/internal/config"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/zerodha/logf"
	"gorm.io/gorm"
)

// CallSession represents an active call with its WebRTC state
type CallSession struct {
	ID              string // WhatsApp call_id
	OrganizationID  uuid.UUID
	AccountName     string
	CallerPhone     string
	ContactID       uuid.UUID
	CallLogID       uuid.UUID
	Status          models.CallStatus
	PeerConnection  *webrtc.PeerConnection
	AudioTrack      *webrtc.TrackLocalStaticRTP
	CurrentMenu     *IVRMenuNode
	IVRFlow         *models.IVRFlow
	DTMFBuffer      chan byte
	StartedAt       time.Time

	// Transfer fields
	TransferID        uuid.UUID
	TransferStatus    models.CallTransferStatus
	AgentPC           *webrtc.PeerConnection
	AgentAudioTrack   *webrtc.TrackLocalStaticRTP
	CallerRemoteTrack *webrtc.TrackRemote
	AgentRemoteTrack  *webrtc.TrackRemote
	Bridge            *AudioBridge
	HoldPlayer        *AudioPlayer
	TransferCancel    context.CancelFunc
	BridgeStarted     chan struct{} // closed when bridge takes over caller track

	// Outgoing call fields
	Direction      models.CallDirection
	AgentID        uuid.UUID
	TargetPhone    string
	WAPeerConn     *webrtc.PeerConnection           // WhatsApp-side PC (outgoing only)
	WAAudioTrack   *webrtc.TrackLocalStaticRTP       // server→WhatsApp audio track
	WARemoteTrack  *webrtc.TrackRemote               // WhatsApp's remote audio track
	SDPAnswerReady chan string                        // webhook delivers SDP answer here

	mu sync.Mutex
}

// IVRMenuNode represents a node in the IVR menu tree (parsed from JSONB)
type IVRMenuNode struct {
	Greeting            string                 `json:"greeting"`
	Options             map[string]IVROption   `json:"options"`
	TimeoutSeconds      int                    `json:"timeout_seconds"`
	MaxRetries          int                    `json:"max_retries"`
	InvalidInputMessage string                 `json:"invalid_input_message"`
	Parent              *IVRMenuNode           `json:"-"`
}

// IVROption represents a single option in an IVR menu
type IVROption struct {
	Label  string       `json:"label"`
	Action string       `json:"action"` // transfer, submenu, repeat, parent, hangup, goto_flow
	Target string       `json:"target,omitempty"`
	Menu   *IVRMenuNode `json:"menu,omitempty"`
}

// Manager manages active call sessions
type Manager struct {
	sessions map[string]*CallSession
	mu       sync.RWMutex
	log      logf.Logger
	whatsapp *whatsapp.Client
	db       *gorm.DB
	wsHub    *websocket.Hub
	config   *config.CallingConfig
}

// NewManager creates a new call session manager
func NewManager(cfg *config.CallingConfig, db *gorm.DB, waClient *whatsapp.Client, wsHub *websocket.Hub, log logf.Logger) *Manager {
	return &Manager{
		sessions: make(map[string]*CallSession),
		log:      log,
		whatsapp: waClient,
		db:       db,
		wsHub:    wsHub,
		config:   cfg,
	}
}

// HandleIncomingCall processes a new incoming call with optional SDP offer
func (m *Manager) HandleIncomingCall(account *models.WhatsAppAccount, contact *models.Contact, callLog *models.CallLog, sdpOffer string) {
	session := &CallSession{
		ID:             callLog.WhatsAppCallID,
		OrganizationID: account.OrganizationID,
		AccountName:    account.Name,
		CallerPhone:    contact.PhoneNumber,
		ContactID:      contact.ID,
		CallLogID:      callLog.ID,
		Status:         models.CallStatusRinging,
		DTMFBuffer:     make(chan byte, 32),
		StartedAt:      time.Now(),
		BridgeStarted:  make(chan struct{}),
	}

	// Load IVR flow if assigned
	if callLog.IVRFlowID != nil {
		var flow models.IVRFlow
		if err := m.db.First(&flow, callLog.IVRFlowID).Error; err == nil {
			session.IVRFlow = &flow
		}
	}

	m.mu.Lock()
	m.sessions[session.ID] = session
	m.mu.Unlock()

	m.log.Info("Call session created",
		"call_id", session.ID,
		"caller", session.CallerPhone,
		"has_sdp", sdpOffer != "",
	)

	// If SDP offer is provided, initiate WebRTC negotiation
	if sdpOffer != "" {
		go m.negotiateWebRTC(session, account, sdpOffer)
	}
}

// HandleCallEvent processes a call lifecycle event (in_call, ended, etc.)
func (m *Manager) HandleCallEvent(callID, event string) {
	m.mu.RLock()
	session, exists := m.sessions[callID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	switch event {
	case "in_call":
		session.Status = models.CallStatusAnswered
	case "ended", "missed", "unanswered":
		// If caller hangs up during a transfer, mark it as abandoned
		if session.TransferStatus == models.CallTransferStatusWaiting {
			session.mu.Unlock()
			m.HandleCallerHangupDuringTransfer(session)
			return
		}
		// If caller hangs up during a connected transfer, end it
		if session.TransferStatus == models.CallTransferStatusConnected {
			transferID := session.TransferID
			session.mu.Unlock()
			m.EndTransfer(transferID)
			return
		}
		session.Status = models.CallStatusCompleted
		go m.cleanupSession(callID)
	}
}

// EndCall terminates a call session and cleans up resources
func (m *Manager) EndCall(callID string) {
	m.cleanupSession(callID)
}

// GetSession returns a call session by ID
func (m *Manager) GetSession(callID string) *CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[callID]
}

// GetSessionByCallLogID returns a call session by its CallLog ID
func (m *Manager) GetSessionByCallLogID(callLogID uuid.UUID) *CallSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, s := range m.sessions {
		if s.CallLogID == callLogID {
			return s
		}
	}
	return nil
}

// cleanupSession removes a session and releases WebRTC resources
func (m *Manager) cleanupSession(callID string) {
	m.mu.Lock()
	session, exists := m.sessions[callID]
	if exists {
		delete(m.sessions, callID)
	}
	m.mu.Unlock()

	if !exists {
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// Stop transfer resources
	if session.Bridge != nil {
		session.Bridge.Stop()
	}
	if session.HoldPlayer != nil {
		session.HoldPlayer.Stop()
	}
	if session.TransferCancel != nil {
		session.TransferCancel()
	}
	if session.AgentPC != nil {
		if err := session.AgentPC.Close(); err != nil {
			m.log.Error("Failed to close agent peer connection", "error", err, "call_id", callID)
		}
	}

	// Close WhatsApp peer connection (outgoing calls)
	if session.WAPeerConn != nil {
		if err := session.WAPeerConn.Close(); err != nil {
			m.log.Error("Failed to close WA peer connection", "error", err, "call_id", callID)
		}
	}

	// Close caller peer connection
	if session.PeerConnection != nil {
		if err := session.PeerConnection.Close(); err != nil {
			m.log.Error("Failed to close peer connection", "error", err, "call_id", callID)
		}
	}

	// Close DTMF buffer channel
	if session.DTMFBuffer != nil {
		close(session.DTMFBuffer)
	}

	m.log.Info("Call session cleaned up", "call_id", callID)
}
