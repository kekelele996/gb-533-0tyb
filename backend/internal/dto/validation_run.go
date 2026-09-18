package dto

import (
	"encoding/json"
	"time"
)

type CollisionEvent struct {
	SegmentIndex    int     `json:"segment_index"`
	FirstTimeMS     float64 `json:"first_time_ms"`
	ZoneID          uint    `json:"zone_id"`
	ZoneName        string  `json:"zone_name"`
	ZoneType        string  `json:"zone_type"`
	AllowedSpeedMMS float64 `json:"allowed_speed_mm_s"`
	ActualSpeedMMS  float64 `json:"actual_speed_mm_s"`
	ClearanceMM     float64 `json:"clearance_mm"`
	Violation       bool    `json:"violation"`
	Evidence        string  `json:"evidence"`
}

type InterlockFinding struct {
	Code      string   `json:"code"`
	Event     string   `json:"event"`
	DependsOn string   `json:"depends_on,omitempty"`
	Path      []string `json:"path,omitempty"`
	Evidence  string   `json:"evidence"`
}

type CreateValidationRunRequest struct {
	MotionProgramID uint `json:"motion_program_id" validate:"required"`
	RetryFailed     bool `json:"retry_failed"`
}

type ReviewValidationRequest struct {
	Note string `json:"note" validate:"required,min=8,max=1000"`
}

// WaiverFindingKind identifies which frozen evidence list a waiver targets.
const (
	WaiverFindingEnvelope  = "envelope_violation"
	WaiverFindingInterlock = "interlock_finding"
)

type GrantWaiverRequest struct {
	FindingKind  string    `json:"finding_kind" validate:"required,oneof=envelope_violation interlock_finding"`
	FindingIndex int       `json:"finding_index" validate:"min=0"`
	Reason       string    `json:"reason" validate:"required,min=8,max=1000"`
	ExpiresAt    time.Time `json:"expires_at" validate:"required"`
}

// GrantWaiversRequest appends one or more waivers atomically; either every
// waiver is persisted or none is.
type GrantWaiversRequest struct {
	Waivers []GrantWaiverRequest `json:"waivers" validate:"required,min=1,max=200,dive"`
}

// AcceptValidationRequest keeps the existing acceptance note and optionally
// appends waivers in the same transaction as the status transition.
type AcceptValidationRequest struct {
	Note    string               `json:"note" validate:"required,min=8,max=1000"`
	Waivers []GrantWaiverRequest `json:"waivers" validate:"omitempty,max=200,dive"`
}

type ViolationWaiverResponse struct {
	ID            uint      `json:"id"`
	FindingKind   string    `json:"finding_kind"`
	FindingIndex  int       `json:"finding_index"`
	Reason        string    `json:"reason"`
	ExpiresAt     time.Time `json:"expires_at"`
	GrantedBy     uint      `json:"granted_by"`
	GrantedByName string    `json:"granted_by_name"`
	GrantedAt     time.Time `json:"granted_at"`
	Active        bool      `json:"active"`
}

type ValidationRunResponse struct {
	ID                uint                      `json:"id"`
	MotionProgramID   uint                      `json:"motion_program_id"`
	ProgramCode       string                    `json:"program_code"`
	ProgramVersion    int                       `json:"program_version"`
	ProgramUploadedBy uint                      `json:"program_uploaded_by"`
	ZoneSnapshot      json.RawMessage           `json:"zone_snapshot"`
	ProgramSnapshot   json.RawMessage           `json:"program_snapshot"`
	AlgorithmVersion  string                    `json:"algorithm_version"`
	InputHash         string                    `json:"input_hash"`
	IdempotencyKey    string                    `json:"idempotency_key"`
	Attempt           int                       `json:"attempt"`
	RetryOfID         *uint                     `json:"retry_of_id"`
	CollisionEvents   []CollisionEvent          `json:"collision_events"`
	InterlockFindings []InterlockFinding        `json:"interlock_findings"`
	ViolationWaivers  []ViolationWaiverResponse `json:"violation_waivers"`
	RiskScore         float64                   `json:"risk_score"`
	ValidationStatus  string                    `json:"validation_status"`
	Explanation       string                    `json:"explanation"`
	RequestedBy       uint                      `json:"requested_by"`
	StartedAt         time.Time                 `json:"started_at"`
	FinishedAt        *time.Time                `json:"finished_at"`
	ReviewedBy        *uint                     `json:"reviewed_by"`
	ReviewedAt        *time.Time                `json:"reviewed_at"`
	ReviewNote        string                    `json:"review_note"`
	Reused            bool                      `json:"reused"`
}
