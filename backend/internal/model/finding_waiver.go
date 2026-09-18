package model

import "time"

// FindingWaiver is a reviewer-supplied, time-bounded exemption for a single
// violation recorded on a validation run. Rows are append-only: when a waiver
// expires a new one may be granted and the expired row remains as history.
// "Effective" is computed as ExpiresAt in the future; at most one effective
// waiver may exist per (run, finding) pair, which the service enforces inside a
// transaction.
type FindingWaiver struct {
	ID              uint          `gorm:"primaryKey"`
	ValidationRunID uint          `gorm:"index:idx_waiver_run;not null"`
	FindingType     string        `gorm:"size:24;index:idx_waiver_finding,priority:1;not null"`
	FindingIndex    int           `gorm:"index:idx_waiver_finding,priority:2;not null"`
	Reason          string        `gorm:"type:text;not null"`
	ExpiresAt       time.Time     `gorm:"not null;index"`
	GrantedBy       uint          `gorm:"index;not null"`
	GrantedByName   string        `gorm:"size:80;not null"`
	GrantedAt       time.Time     `gorm:"index;not null"`
	ValidationRun   ValidationRun `gorm:"foreignKey:ValidationRunID"`
}
