package handlers

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/contactutil"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/internal/websocket"
)

// processCallWebhook handles a call webhook event for both incoming and outgoing calls.
// It creates/updates the CallLog and delegates to the CallManager for WebRTC handling.
func (a *App) processCallWebhook(phoneNumberID string, call interface{}) {
	// The webhook handler passes an anonymous struct. Convert via JSON round-trip.
	type callEvent struct {
		ID        string `json:"id"`
		From      string `json:"from"`
		Timestamp string `json:"timestamp"`
		Type      string `json:"type"`
		Event     string `json:"event"`
		SDP       string `json:"sdp,omitempty"`
		Error     *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	var ce callEvent
	b, _ := json.Marshal(call)
	if err := json.Unmarshal(b, &ce); err != nil {
		a.Log.Error("Failed to parse call event", "error", err)
		return
	}

	// Check if this call_id belongs to an existing outgoing session
	if a.CallManager != nil {
		session := a.CallManager.GetSession(ce.ID)
		if session != nil && session.Direction == models.CallDirectionOutgoing {
			a.CallManager.HandleOutgoingCallWebhook(ce.ID, ce.Event, ce.SDP)
			return
		}
	}

	// --- Incoming call flow ---

	// Look up the WhatsApp account
	account, err := a.getWhatsAppAccountCached(phoneNumberID)
	if err != nil {
		a.Log.Error("Failed to find WhatsApp account for call", "error", err, "phone_id", phoneNumberID)
		return
	}

	// Get or create the contact
	contact, _, _ := contactutil.GetOrCreateContact(a.DB, account.OrganizationID, ce.From, "")

	if contact == nil {
		a.Log.Error("Failed to get or create contact for call", "phone", ce.From)
		return
	}

	now := time.Now()

	switch ce.Event {
	case "ringing":
		// Create a new call log
		callLog := models.CallLog{
			BaseModel:       models.BaseModel{ID: uuid.New()},
			OrganizationID:  account.OrganizationID,
			WhatsAppAccount: account.Name,
			ContactID:       contact.ID,
			WhatsAppCallID:  ce.ID,
			CallerPhone:     ce.From,
			Status:          models.CallStatusRinging,
			StartedAt:       &now,
		}

		// Find active IVR flow for this account
		var ivrFlow models.IVRFlow
		if err := a.DB.Where("organization_id = ? AND whatsapp_account = ? AND is_active = ? AND deleted_at IS NULL",
			account.OrganizationID, account.Name, true).First(&ivrFlow).Error; err == nil {
			callLog.IVRFlowID = &ivrFlow.ID
		}

		if err := a.DB.Create(&callLog).Error; err != nil {
			a.Log.Error("Failed to create call log", "error", err)
			return
		}

		// Delegate to CallManager for WebRTC handling if enabled
		if a.CallManager != nil {
			a.CallManager.HandleIncomingCall(account, contact, &callLog, ce.SDP)
		}

		// Broadcast incoming call via WebSocket
		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallIncoming, map[string]any{
			"call_log_id":  callLog.ID.String(),
			"call_id":      ce.ID,
			"caller_phone": ce.From,
			"contact_id":   contact.ID.String(),
			"contact_name": contact.ProfileName,
			"ivr_flow_id":  callLog.IVRFlowID,
			"started_at":   now.Format(time.RFC3339),
		})

	case "in_call":
		// Update call status to answered
		a.DB.Model(&models.CallLog{}).
			Where("whatsapp_call_id = ? AND organization_id = ?", ce.ID, account.OrganizationID).
			Updates(map[string]any{
				"status":      models.CallStatusAnswered,
				"answered_at": now,
			})

		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallAnswered, map[string]any{
			"call_id":     ce.ID,
			"contact_id":  contact.ID.String(),
			"answered_at": now.Format(time.RFC3339),
		})

	case "ended":
		// Calculate duration and update
		var callLog models.CallLog
		if err := a.DB.Where("whatsapp_call_id = ? AND organization_id = ?", ce.ID, account.OrganizationID).
			First(&callLog).Error; err != nil {
			a.Log.Error("Call log not found for ended event", "call_id", ce.ID)
			return
		}

		duration := 0
		if callLog.AnsweredAt != nil {
			duration = int(now.Sub(*callLog.AnsweredAt).Seconds())
		}

		a.DB.Model(&callLog).Updates(map[string]any{
			"status":   models.CallStatusCompleted,
			"ended_at": now,
			"duration": duration,
		})

		// Notify CallManager to clean up
		if a.CallManager != nil {
			a.CallManager.EndCall(ce.ID)
		}

		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallEnded, map[string]any{
			"call_id":    ce.ID,
			"contact_id": contact.ID.String(),
			"duration":   duration,
			"ended_at":   now.Format(time.RFC3339),
		})

	case "missed", "unanswered":
		a.DB.Model(&models.CallLog{}).
			Where("whatsapp_call_id = ? AND organization_id = ?", ce.ID, account.OrganizationID).
			Updates(map[string]any{
				"status":   models.CallStatusMissed,
				"ended_at": now,
			})

		a.broadcastCallEvent(account.OrganizationID, websocket.TypeCallEnded, map[string]any{
			"call_id":    ce.ID,
			"contact_id": contact.ID.String(),
			"status":     string(models.CallStatusMissed),
			"ended_at":   now.Format(time.RFC3339),
		})

	default:
		a.Log.Warn("Unknown call event", "event", ce.Event, "call_id", ce.ID)
	}

	// Handle error in call event
	if ce.Error != nil {
		a.DB.Model(&models.CallLog{}).
			Where("whatsapp_call_id = ? AND organization_id = ?", ce.ID, account.OrganizationID).
			Updates(map[string]any{
				"status":        models.CallStatusFailed,
				"error_message": ce.Error.Message,
				"ended_at":      now,
			})
	}
}

// broadcastCallEvent sends a call event to all connected clients in an organization
func (a *App) broadcastCallEvent(orgID uuid.UUID, eventType string, payload map[string]any) {
	if a.WSHub == nil {
		return
	}
	a.WSHub.BroadcastToOrg(orgID, websocket.WSMessage{
		Type:    eventType,
		Payload: payload,
	})
}
