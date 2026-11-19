package domain

import "github.com/google/uuid"

type RegisterRequest struct {
	CompanyName string `json:"company_name" binding:"required,min=2,max=180"`
	Name        string `json:"name" binding:"required,min=2,max=180"`
	Email       string `json:"email" binding:"required,email,max=254"`
	Password    string `json:"password" binding:"required,min=8,max=72"`
}

type LoginRequest struct {
	Email     string     `json:"email" binding:"required,email"`
	Password  string     `json:"password" binding:"required"`
	CompanyID *uuid.UUID `json:"company_id"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type AuthResponse struct {
	AccessToken  string     `json:"access_token"`
	RefreshToken string     `json:"refresh_token"`
	TokenType    string     `json:"token_type"`
	ExpiresIn    int64      `json:"expires_in"`
	User         User       `json:"user"`
	Membership   Membership `json:"membership"`
}

type UpdateUserRequest struct {
	Name string `json:"name" binding:"required,min=2,max=180"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8,max=72"`
}

type UpdateCompanyRequest struct {
	Name   *string `json:"name" binding:"omitempty,min=2,max=180"`
	Status *string `json:"status" binding:"omitempty,oneof=active inactive"`
}

type LocationRequest struct {
	Code   string `json:"code" binding:"required,min=1,max=50"`
	Name   string `json:"name" binding:"required,min=2,max=180"`
	Status string `json:"status" binding:"omitempty,oneof=active inactive"`
}

type LocationUpdateRequest struct {
	Code   *string `json:"code" binding:"omitempty,min=1,max=50"`
	Name   *string `json:"name" binding:"omitempty,min=2,max=180"`
	Status *string `json:"status" binding:"omitempty,oneof=active inactive"`
}

type PatientRequest struct {
	PrimaryLocationID *uuid.UUID `json:"primary_location_id"`
	PublicRef         string     `json:"public_ref" binding:"required,min=1,max=100"`
	ExternalChartID   string     `json:"external_chart_id" binding:"max=100"`
	Status            string     `json:"status" binding:"omitempty,oneof=active inactive"`
}

type PatientUpdateRequest struct {
	PrimaryLocationID *uuid.UUID `json:"primary_location_id"`
	PublicRef         *string    `json:"public_ref" binding:"omitempty,min=1,max=100"`
	ExternalChartID   *string    `json:"external_chart_id" binding:"omitempty,max=100"`
	Status            *string    `json:"status" binding:"omitempty,oneof=active inactive"`
}

type PatientLocationRequest struct {
	LocationID   uuid.UUID `json:"location_id" binding:"required"`
	Relationship string    `json:"relationship" binding:"required,min=1,max=50"`
}

type MembershipRequest struct {
	Email    string    `json:"email" binding:"required,email"`
	Name     string    `json:"name" binding:"omitempty,min=2,max=180"`
	Password string    `json:"password" binding:"omitempty,min=8,max=72"`
	RoleID   uuid.UUID `json:"role_id" binding:"required"`
	Status   string    `json:"status" binding:"omitempty,oneof=active inactive pending"`
}

type MembershipUpdateRequest struct {
	RoleID *uuid.UUID `json:"role_id"`
	Status *string    `json:"status" binding:"omitempty,oneof=active inactive pending"`
}

type RoleRequest struct {
	Code        string `json:"code" binding:"required,min=2,max=50"`
	Scope       string `json:"scope" binding:"required,oneof=company location"`
	Description string `json:"description" binding:"max=255"`
}

type AssignmentRequest struct {
	LocationID uuid.UUID `json:"location_id" binding:"required"`
}

type OrderRequest struct {
	LocationID      uuid.UUID  `json:"location_id" binding:"required"`
	PatientID       uuid.UUID  `json:"patient_id" binding:"required"`
	JourneyStepID   *uuid.UUID `json:"journey_step_id"`
	ExternalOrderID string     `json:"external_order_id" binding:"required,min=1,max=100"`
	Status          string     `json:"status" binding:"omitempty,oneof=pending processing completed cancelled failed"`
}

type OrderUpdateRequest struct {
	LocationID      *uuid.UUID `json:"location_id"`
	PatientID       *uuid.UUID `json:"patient_id"`
	JourneyStepID   *uuid.UUID `json:"journey_step_id"`
	ExternalOrderID *string    `json:"external_order_id" binding:"omitempty,min=1,max=100"`
	Status          *string    `json:"status" binding:"omitempty,oneof=pending processing completed cancelled failed"`
}

type PatientDetail struct {
	Patient   Patient           `json:"patient"`
	Locations []PatientLocation `json:"locations"`
}

type Page[T any] struct {
	Items      []T   `json:"items"`
	Page       int   `json:"page"`
	PageSize   int   `json:"page_size"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"total_pages"`
}
