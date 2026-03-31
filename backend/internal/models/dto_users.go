package models

import "time"

// UpdateUserRoleRequest is the request body for updating a user's role.
type UpdateUserRoleRequest struct {
	Role string `json:"role"`
}

// CreateUserRequest is the request body for creating a user.
type CreateUserRequest struct {
	Login string `json:"login"`
}

// UserResponse is the API response shape for a user.
type UserResponse struct {
	ID          uint      `json:"id"`
	Login       string    `json:"login"`
	Role        string    `json:"role"`
	LastLoginAt time.Time `json:"lastLoginAt"`
}

// NewUserResponse builds a UserResponse from a User model.
func NewUserResponse(user *User) UserResponse {
	return UserResponse{
		ID:          user.ID,
		Login:       user.Login,
		Role:        user.Role,
		LastLoginAt: user.LastLoginAt,
	}
}

// NewUserResponses builds a slice of UserResponse from User models.
func NewUserResponses(users []User) []UserResponse {
	response := make([]UserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, NewUserResponse(&user))
	}
	return response
}
