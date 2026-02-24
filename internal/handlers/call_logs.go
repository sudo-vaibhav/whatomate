package handlers

import (
	"time"

	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
)

// ListCallLogs returns call logs for the organization
func (a *App) ListCallLogs(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceCallLogs, models.ActionRead); err != nil {
		return nil
	}

	pg := parsePagination(r)
	status := string(r.RequestCtx.QueryArgs().Peek("status"))
	account := string(r.RequestCtx.QueryArgs().Peek("account"))
	contactIDStr := string(r.RequestCtx.QueryArgs().Peek("contact_id"))
	direction := string(r.RequestCtx.QueryArgs().Peek("direction"))
	ivrFlowID := string(r.RequestCtx.QueryArgs().Peek("ivr_flow_id"))

	query := a.DB.Where("call_logs.organization_id = ?", orgID).
		Preload("Contact").
		Preload("IVRFlow").
		Order("call_logs.created_at DESC")

	countQuery := a.DB.Model(&models.CallLog{}).Where("organization_id = ?", orgID)

	if status != "" {
		query = query.Where("call_logs.status = ?", status)
		countQuery = countQuery.Where("status = ?", status)
	}
	if account != "" {
		query = query.Where("call_logs.whatsapp_account = ?", account)
		countQuery = countQuery.Where("whatsapp_account = ?", account)
	}
	if contactIDStr != "" {
		query = query.Where("call_logs.contact_id = ?", contactIDStr)
		countQuery = countQuery.Where("contact_id = ?", contactIDStr)
	}
	if direction != "" {
		query = query.Where("call_logs.direction = ?", direction)
		countQuery = countQuery.Where("direction = ?", direction)
	}
	if ivrFlowID != "" {
		query = query.Where("call_logs.ivr_flow_id = ?", ivrFlowID)
		countQuery = countQuery.Where("ivr_flow_id = ?", ivrFlowID)
	}

	// Date range filter
	if start, ok := parseDateParam(r, "start_date"); ok {
		query = query.Where("call_logs.created_at >= ?", start)
		countQuery = countQuery.Where("created_at >= ?", start)
	}
	if end, ok := parseDateParam(r, "end_date"); ok {
		query = query.Where("call_logs.created_at <= ?", endOfDay(end))
		countQuery = countQuery.Where("created_at <= ?", endOfDay(end))
	}

	var total int64
	countQuery.Count(&total)

	var callLogs []models.CallLog
	if err := pg.Apply(query).Find(&callLogs).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to fetch call logs", nil, "")
	}

	// Mask phone numbers if enabled for this organization
	if a.ShouldMaskPhoneNumbers(orgID) {
		for i := range callLogs {
			callLogs[i].CallerPhone = MaskPhoneNumber(callLogs[i].CallerPhone)
			if callLogs[i].Contact != nil {
				callLogs[i].Contact.PhoneNumber = MaskPhoneNumber(callLogs[i].Contact.PhoneNumber)
				callLogs[i].Contact.ProfileName = MaskIfPhoneNumber(callLogs[i].Contact.ProfileName)
			}
		}
	}

	return r.SendEnvelope(map[string]any{
		"call_logs": callLogs,
		"total":     total,
		"page":      pg.Page,
		"limit":     pg.Limit,
	})
}

// GetCallLog returns a single call log by ID
func (a *App) GetCallLog(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceCallLogs, models.ActionRead); err != nil {
		return nil
	}

	logID, err := parsePathUUID(r, "id", "call log")
	if err != nil {
		return nil
	}

	var callLog models.CallLog
	if err := a.DB.Where("id = ? AND organization_id = ?", logID, orgID).
		Preload("Contact").
		Preload("IVRFlow").
		First(&callLog).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Call log not found", nil, "")
	}

	if a.ShouldMaskPhoneNumbers(orgID) {
		callLog.CallerPhone = MaskPhoneNumber(callLog.CallerPhone)
		if callLog.Contact != nil {
			callLog.Contact.PhoneNumber = MaskPhoneNumber(callLog.Contact.PhoneNumber)
			callLog.Contact.ProfileName = MaskIfPhoneNumber(callLog.Contact.ProfileName)
		}
	}

	return r.SendEnvelope(callLog)
}

// GetCallRecording returns a presigned S3 URL for a call recording.
func (a *App) GetCallRecording(r *fastglue.Request) error {
	orgID, userID, err := a.getOrgAndUserID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}
	if err := a.requirePermission(r, userID, models.ResourceCallLogs, models.ActionRead); err != nil {
		return nil
	}

	if a.S3Client == nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Recording not available", nil, "")
	}

	logID, err := parsePathUUID(r, "id", "call log")
	if err != nil {
		return nil
	}

	var callLog models.CallLog
	if err := a.DB.Where("id = ? AND organization_id = ?", logID, orgID).
		First(&callLog).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "Call log not found", nil, "")
	}

	if callLog.RecordingS3Key == "" {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "No recording for this call", nil, "")
	}

	url, err := a.S3Client.GetPresignedURL(r.RequestCtx, callLog.RecordingS3Key, 15*time.Minute)
	if err != nil {
		a.Log.Error("Failed to generate presigned URL", "error", err, "call_log_id", logID)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to generate recording URL", nil, "")
	}

	return r.SendEnvelope(map[string]any{
		"url":      url,
		"duration": callLog.RecordingDuration,
	})
}
