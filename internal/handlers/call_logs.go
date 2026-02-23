package handlers

import (
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

	return r.SendEnvelope(callLog)
}
