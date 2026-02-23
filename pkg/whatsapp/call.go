package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// buildCallsURL builds the calls endpoint URL for the WhatsApp Calling API
func (c *Client) buildCallsURL(account *Account) string {
	return fmt.Sprintf("%s/%s/%s/calls", c.getBaseURL(), account.APIVersion, account.PhoneID)
}

// PreAcceptCall signals to Meta that the server is ready to accept the call.
// This should be sent before AcceptCall to keep the call alive while SDP is being prepared.
func (c *Client) PreAcceptCall(ctx context.Context, account *Account, callID string) error {
	payload := map[string]string{
		"messaging_product": "whatsapp",
		"call_id":           callID,
		"action":            "pre_accept",
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Pre-accepting call", "call_id", callID)

	_, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to pre-accept call: %w", err)
	}

	c.Log.Info("Call pre-accepted", "call_id", callID)
	return nil
}

// AcceptCall accepts an incoming call with an SDP answer.
func (c *Client) AcceptCall(ctx context.Context, account *Account, callID, sdpAnswer string) error {
	payload := map[string]string{
		"messaging_product": "whatsapp",
		"call_id":           callID,
		"action":            "accept",
		"sdp":               sdpAnswer,
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Accepting call", "call_id", callID)

	_, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to accept call: %w", err)
	}

	c.Log.Info("Call accepted", "call_id", callID)
	return nil
}

// RejectCall rejects an incoming call.
func (c *Client) RejectCall(ctx context.Context, account *Account, callID string) error {
	payload := map[string]string{
		"messaging_product": "whatsapp",
		"call_id":           callID,
		"action":            "reject",
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Rejecting call", "call_id", callID)

	_, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to reject call: %w", err)
	}

	c.Log.Info("Call rejected", "call_id", callID)
	return nil
}

// SendCallPermissionRequest sends an interactive call_permission_request message
// to the consumer. The consumer must accept before outgoing calls can be placed.
// Permission is valid for 72 hours once accepted.
func (c *Client) SendCallPermissionRequest(ctx context.Context, account *Account, phoneNumber, bodyText string) (string, error) {
	if bodyText == "" {
		bodyText = "We'd like to call you to assist with your query."
	}

	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                phoneNumber,
		"type":              "interactive",
		"interactive": map[string]interface{}{
			"type": "call_permission_request",
			"body": map[string]string{
				"text": bodyText,
			},
			"action": map[string]interface{}{
				"name":       "voice_call",
				"parameters": map[string]string{},
			},
		},
	}

	url := c.buildMessagesURL(account)
	c.Log.Info("Sending call permission request", "phone", phoneNumber)

	respBody, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to send call permission request: %w", err)
	}

	// Parse message ID from response
	var resp struct {
		Messages []struct {
			ID string `json:"id"`
		} `json:"messages"`
	}
	if parseErr := json.Unmarshal(respBody, &resp); parseErr == nil && len(resp.Messages) > 0 {
		c.Log.Info("Call permission request sent", "phone", phoneNumber, "message_id", resp.Messages[0].ID)
		return resp.Messages[0].ID, nil
	}

	return "", nil
}

// InitiateCall places an outgoing call to a WhatsApp user with an SDP offer.
// Returns the call_id assigned by WhatsApp on success.
func (c *Client) InitiateCall(ctx context.Context, account *Account, phoneNumber, sdpOffer string) (string, error) {
	payload := map[string]interface{}{
		"messaging_product": "whatsapp",
		"to":                phoneNumber,
		"type":              "voice",
		"sdp":               sdpOffer,
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Initiating outgoing call", "phone", phoneNumber)

	respBody, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return "", fmt.Errorf("failed to initiate call: %w", err)
	}

	// Parse call_id from response
	var resp struct {
		CallID string `json:"call_id"`
	}
	if parseErr := json.Unmarshal(respBody, &resp); parseErr != nil || resp.CallID == "" {
		return "", fmt.Errorf("failed to parse call_id from response")
	}

	c.Log.Info("Outgoing call initiated", "phone", phoneNumber, "call_id", resp.CallID)
	return resp.CallID, nil
}

// TerminateCall terminates an active call.
func (c *Client) TerminateCall(ctx context.Context, account *Account, callID string) error {
	payload := map[string]string{
		"messaging_product": "whatsapp",
		"call_id":           callID,
		"action":            "terminate",
	}

	url := c.buildCallsURL(account)
	c.Log.Info("Terminating call", "call_id", callID)

	_, err := c.doRequest(ctx, http.MethodPost, url, payload, account.AccessToken)
	if err != nil {
		return fmt.Errorf("failed to terminate call: %w", err)
	}

	c.Log.Info("Call terminated", "call_id", callID)
	return nil
}
