package handlers

import (
	"time"

	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/shridarpatil/whatomate/pkg/whatsapp"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// InitiateOutgoingCall handles POST /api/calls/outgoing
// Lets an agent start a voice call to a WhatsApp consumer.
func (a *App) InitiateOutgoingCall(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceOutgoingCalls, models.ActionWrite); err != nil {
		return nil
	}

	var req struct {
		ContactPhone    string `json:"contact_phone"`
		WhatsAppAccount string `json:"whatsapp_account"`
		SDPOffer        string `json:"sdp_offer"`
	}
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.ContactPhone == "" || req.WhatsAppAccount == "" || req.SDPOffer == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "contact_phone, whatsapp_account, and sdp_offer are required", nil, "")
	}

	if a.CallManager == nil {
		return r.SendErrorEnvelope(fasthttp.StatusServiceUnavailable, "Calling is not enabled", nil, "")
	}

	// Look up account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.WhatsAppAccount).
		First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	// Look up contact by phone
	var contact models.Contact
	if err := a.DB.Where("organization_id = ? AND phone_number = ?", orgID, req.ContactPhone).
		First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	callLogID, sdpAnswer, err := a.CallManager.InitiateOutgoingCall(
		orgID, userID, contact.ID,
		req.ContactPhone, req.WhatsAppAccount,
		waAccount, req.SDPOffer,
	)
	if err != nil {
		a.Log.Error("Failed to initiate outgoing call", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to initiate call: "+err.Error(), nil, "")
	}

	return r.SendEnvelope(map[string]string{
		"call_log_id": callLogID.String(),
		"sdp_answer":  sdpAnswer,
	})
}

// HangupOutgoingCall handles POST /api/calls/outgoing/{id}/hangup
func (a *App) HangupOutgoingCall(r *fastglue.Request) error {
	_, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceOutgoingCalls, models.ActionWrite); err != nil {
		return nil
	}

	callLogID, err := parsePathUUID(r, "id", "call log")
	if err != nil {
		return nil
	}

	if a.CallManager == nil {
		return r.SendErrorEnvelope(fasthttp.StatusServiceUnavailable, "Calling is not enabled", nil, "")
	}

	if err := a.CallManager.HangupOutgoingCall(callLogID, userID); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, err.Error(), nil, "")
	}

	return r.SendEnvelope(map[string]string{"status": "ok"})
}

// SendCallPermissionRequest handles POST /api/calls/permission-request
func (a *App) SendCallPermissionRequest(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceOutgoingCalls, models.ActionWrite); err != nil {
		return nil
	}

	var req struct {
		ContactID       string `json:"contact_id"`
		WhatsAppAccount string `json:"whatsapp_account"`
	}
	if err := a.decodeRequest(r, &req); err != nil {
		return nil
	}

	if req.ContactID == "" || req.WhatsAppAccount == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "contact_id and whatsapp_account are required", nil, "")
	}

	contactID, parseErr := uuid.Parse(req.ContactID)
	if parseErr != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid contact_id", nil, "")
	}

	// Verify contact exists
	var contact models.Contact
	if err := a.DB.Where("id = ? AND organization_id = ?", contactID, orgID).First(&contact).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Contact not found", nil, "")
	}

	// Look up account
	var account models.WhatsAppAccount
	if err := a.DB.Where("organization_id = ? AND name = ?", orgID, req.WhatsAppAccount).
		First(&account).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "WhatsApp account not found", nil, "")
	}

	waAccount := &whatsapp.Account{
		PhoneID:     account.PhoneID,
		BusinessID:  account.BusinessID,
		APIVersion:  account.APIVersion,
		AccessToken: account.AccessToken,
	}

	// Send permission request via WhatsApp Messages API
	ctx := r.RequestCtx
	messageID, err := a.WhatsApp.SendCallPermissionRequest(ctx, waAccount, contact.PhoneNumber, "")
	if err != nil {
		a.Log.Error("Failed to send call permission request", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to send permission request", nil, "")
	}

	// Create CallPermission record
	permission := models.CallPermission{
		BaseModel:       models.BaseModel{ID: uuid.New()},
		OrganizationID:  orgID,
		ContactID:       contactID,
		WhatsAppAccount: req.WhatsAppAccount,
		Status:          models.CallPermissionPending,
		MessageID:       messageID,
		RequestedByID:   &userID,
	}
	if err := a.DB.Create(&permission).Error; err != nil {
		a.Log.Error("Failed to create call permission record", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to save permission", nil, "")
	}

	return r.SendEnvelope(map[string]string{
		"permission_id": permission.ID.String(),
	})
}

// GetCallPermission handles GET /api/calls/permission/{contactId}
func (a *App) GetCallPermission(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceOutgoingCalls, models.ActionRead); err != nil {
		return nil
	}

	contactID, err := parsePathUUID(r, "contactId", "contact")
	if err != nil {
		return nil
	}

	var permission models.CallPermission
	if err := a.DB.Where("organization_id = ? AND contact_id = ?", orgID, contactID).
		Order("created_at DESC").
		First(&permission).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "No permission found for contact", nil, "")
	}

	// Check if permission has expired (72h from responded_at)
	if permission.Status == models.CallPermissionAccepted && permission.RespondedAt != nil {
		if time.Since(*permission.RespondedAt) > 72*time.Hour {
			permission.Status = models.CallPermissionExpired
			a.DB.Model(&permission).Update("status", models.CallPermissionExpired)
		}
	}

	return r.SendEnvelope(permission)
}
