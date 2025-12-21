package handlers

import (
	"github.com/google/uuid"
	"github.com/shridarpatil/whatomate/internal/models"
	"github.com/valyala/fasthttp"
	"github.com/zerodha/fastglue"
	"golang.org/x/crypto/bcrypt"
)

// UserRequest represents the request body for creating/updating a user
type UserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Role     string `json:"role"`
	IsActive *bool  `json:"is_active"`
}

// UserResponse represents the response for a user (without sensitive data)
type UserResponse struct {
	ID             uuid.UUID `json:"id"`
	Email          string    `json:"email"`
	FullName       string    `json:"full_name"`
	Role           string    `json:"role"`
	IsActive       bool      `json:"is_active"`
	OrganizationID uuid.UUID `json:"organization_id"`
	CreatedAt      string    `json:"created_at"`
	UpdatedAt      string    `json:"updated_at"`
}

// ListUsers returns all users for the organization (admin only)
func (a *App) ListUsers(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check if user is admin
	role, _ := r.RequestCtx.UserValue("role").(string)
	if role != "admin" {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Admin access required", nil, "")
	}

	var users []models.User
	if err := a.DB.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&users).Error; err != nil {
		a.Log.Error("Failed to list users", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to list users", nil, "")
	}

	// Convert to response format (hide sensitive data)
	response := make([]UserResponse, len(users))
	for i, user := range users {
		response[i] = userToResponse(user)
	}

	return r.SendEnvelope(map[string]interface{}{
		"users": response,
	})
}

// GetUser returns a single user
func (a *App) GetUser(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid user ID", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	return r.SendEnvelope(userToResponse(user))
}

// CreateUser creates a new user (admin only)
func (a *App) CreateUser(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check if user is admin
	role, _ := r.RequestCtx.UserValue("role").(string)
	if role != "admin" {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Admin access required", nil, "")
	}

	var req UserRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" || req.FullName == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Email, password, and full_name are required", nil, "")
	}

	// Validate role
	if req.Role == "" {
		req.Role = "agent" // Default role
	}
	if req.Role != "admin" && req.Role != "manager" && req.Role != "agent" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid role. Must be admin, manager, or agent", nil, "")
	}

	// Check if email already exists
	var existingUser models.User
	if err := a.DB.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return r.SendErrorEnvelope(fasthttp.StatusConflict, "Email already exists", nil, "")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		a.Log.Error("Failed to hash password", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create user", nil, "")
	}

	user := models.User{
		OrganizationID: orgID,
		Email:          req.Email,
		PasswordHash:   string(hashedPassword),
		FullName:       req.FullName,
		Role:           req.Role,
		IsActive:       true,
	}

	if err := a.DB.Create(&user).Error; err != nil {
		a.Log.Error("Failed to create user", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to create user", nil, "")
	}

	return r.SendEnvelope(userToResponse(user))
}

// UpdateUser updates a user (admin only for role changes)
func (a *App) UpdateUser(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	currentUserID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	currentRole, _ := r.RequestCtx.UserValue("role").(string)

	idStr, ok := r.RequestCtx.UserValue("id").(string)
	if !ok || idStr == "" {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Missing user ID", nil, "")
	}
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid user ID", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	var req UserRequest
	if err := r.Decode(&req, "json"); err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid request body", nil, "")
	}

	// Only admin can update other users or change roles
	if currentRole != "admin" && currentUserID != id {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Admin access required", nil, "")
	}

	// Prevent admin from demoting themselves
	if currentUserID == id && req.Role != "" && req.Role != user.Role {
		if user.Role == "admin" && req.Role != "admin" {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot demote yourself", nil, "")
		}
	}

	// Only admin can change roles
	if req.Role != "" && req.Role != user.Role && currentRole != "admin" {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Admin access required to change roles", nil, "")
	}

	// Update fields if provided
	if req.Email != "" {
		// Check if email already exists for another user
		var existingUser models.User
		if err := a.DB.Where("email = ? AND id != ?", req.Email, id).First(&existingUser).Error; err == nil {
			return r.SendErrorEnvelope(fasthttp.StatusConflict, "Email already exists", nil, "")
		}
		user.Email = req.Email
	}
	if req.FullName != "" {
		user.FullName = req.FullName
	}
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			a.Log.Error("Failed to hash password", "error", err)
			return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update user", nil, "")
		}
		user.PasswordHash = string(hashedPassword)
	}
	if req.Role != "" {
		if req.Role != "admin" && req.Role != "manager" && req.Role != "agent" {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid role. Must be admin, manager, or agent", nil, "")
		}
		user.Role = req.Role
	}
	if req.IsActive != nil {
		// Prevent admin from deactivating themselves
		if currentUserID == id && !*req.IsActive {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot deactivate yourself", nil, "")
		}
		user.IsActive = *req.IsActive
	}

	if err := a.DB.Save(&user).Error; err != nil {
		a.Log.Error("Failed to update user", "error", err)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to update user", nil, "")
	}

	return r.SendEnvelope(userToResponse(user))
}

// DeleteUser deletes a user (admin only)
func (a *App) DeleteUser(r *fastglue.Request) error {
	orgID, err := getOrganizationID(r)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	// Check if user is admin
	currentRole, _ := r.RequestCtx.UserValue("role").(string)
	if currentRole != "admin" {
		return r.SendErrorEnvelope(fasthttp.StatusForbidden, "Admin access required", nil, "")
	}

	currentUserID, _ := r.RequestCtx.UserValue("user_id").(uuid.UUID)

	idStr := r.RequestCtx.UserValue("id").(string)
	id, err := uuid.Parse(idStr)
	if err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Invalid user ID", nil, "")
	}

	// Prevent admin from deleting themselves
	if currentUserID == id {
		return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot delete yourself", nil, "")
	}

	// Check if this is the last admin
	var user models.User
	if err := a.DB.Where("id = ? AND organization_id = ?", id, orgID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	if user.Role == "admin" {
		var adminCount int64
		a.DB.Model(&models.User{}).Where("organization_id = ? AND role = ?", orgID, "admin").Count(&adminCount)
		if adminCount <= 1 {
			return r.SendErrorEnvelope(fasthttp.StatusBadRequest, "Cannot delete the last admin", nil, "")
		}
	}

	result := a.DB.Where("id = ? AND organization_id = ?", id, orgID).Delete(&models.User{})
	if result.Error != nil {
		a.Log.Error("Failed to delete user", "error", result.Error)
		return r.SendErrorEnvelope(fasthttp.StatusInternalServerError, "Failed to delete user", nil, "")
	}
	if result.RowsAffected == 0 {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	return r.SendEnvelope(map[string]string{"message": "User deleted successfully"})
}

// GetCurrentUser returns the current authenticated user's details
func (a *App) GetCurrentUser(r *fastglue.Request) error {
	userID, ok := r.RequestCtx.UserValue("user_id").(uuid.UUID)
	if !ok {
		return r.SendErrorEnvelope(fasthttp.StatusUnauthorized, "Unauthorized", nil, "")
	}

	var user models.User
	if err := a.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		return r.SendErrorEnvelope(fasthttp.StatusNotFound, "User not found", nil, "")
	}

	return r.SendEnvelope(userToResponse(user))
}

// Helper function to convert User to UserResponse
func userToResponse(user models.User) UserResponse {
	return UserResponse{
		ID:             user.ID,
		Email:          user.Email,
		FullName:       user.FullName,
		Role:           user.Role,
		IsActive:       user.IsActive,
		OrganizationID: user.OrganizationID,
		CreatedAt:      user.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:      user.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
