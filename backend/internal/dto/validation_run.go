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

// FindingWaiverRequest is one reviewer exemption for a single violation.
// FindingIndex is the zero-based position inside the run's envelope violation
// list (when FindingType is envelope_violation) or interlock_findings list.
type FindingWaiverRequest struct {
	FindingType  string    `json:"finding_type" validate:"required,oneof=envelope_violation interlock_finding"`
	FindingIndex int       `json:"finding_index" validate:"min=0"`
	Reason       string    `json:"reason" validate:"required,min=8,max=1000"`
	ExpiresAt    time.Time `json:"expires_at" validate:"required"`
}

type GrantWaiverRequest struct {
	Waiver FindingWaiverRequest `json:"waiver" validate:"required"`
}

// AcceptValidationRequest allows new waivers to be appended atomically with the
// acceptance of a failed run. Either all waivers persist and the run reaches
// accepted, or nothing changes and the run stays failed (or reviewed).
type AcceptValidationRequest struct {
	Note    string                 `json:"note" validate:"required,min=8,max=1000"`
	Waivers []FindingWaiverRequest `json:"waivers" validate:"dive"`
}

type FindingWaiverResponse struct {
	ID              uint      `json:"id"`
	ValidationRunID uint      `json:"validation_run_id"`
	FindingType     string    `json:"finding_type"`
	FindingIndex    int       `json:"finding_index"`
	Reason          string    `json:"reason"`
	ExpiresAt       time.Time `json:"expires_at"`
	GrantedBy       uint      `json:"granted_by"`
	GrantedByName   string    `json:"granted_by_name"`
	GrantedAt       time.Time `json:"granted_at"`
}

type ValidationRunResponse struct {
	ID                uint                    `json:"id"`
	MotionProgramID   uint                    `json:"motion_program_id"`
	ProgramCode       string                  `json:"program_code"`
	ProgramVersion    int                     `json:"program_version"`
	ZoneSnapshot      json.RawMessage         `json:"zone_snapshot"`
	ProgramSnapshot   json.RawMessage         `json:"program_snapshot"`
	AlgorithmVersion  string                  `json:"algorithm_version"`
	InputHash         string                  `json:"input_hash"`
	IdempotencyKey    string                  `json:"idempotency_key"`
	Attempt           int                     `json:"attempt"`
	RetryOfID         *uint                   `json:"retry_of_id"`
	CollisionEvents   []CollisionEvent        `json:"collision_events"`
	InterlockFindings []InterlockFinding      `json:"interlock_findings"`
	RiskScore         float64                 `json:"risk_score"`
	ValidationStatus  string                  `json:"validation_status"`
	Explanation       string                  `json:"explanation"`
	RequestedBy       uint                    `json:"requested_by"`
	StartedAt         time.Time               `json:"started_at"`
	FinishedAt        *time.Time              `json:"finished_at"`
	ReviewedBy        *uint                   `json:"reviewed_by"`
	ReviewedAt        *time.Time              `json:"reviewed_at"`
	ReviewNote        string                  `json:"review_note"`
	FindingWaivers    []FindingWaiverResponse `json:"finding_waivers"`
	Reused            bool                    `json:"reused"`
}
