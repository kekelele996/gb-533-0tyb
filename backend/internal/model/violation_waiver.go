package model

import "time"

// ViolationWaiver records a reviewer's time-limited justification for a single
// envelope violation or interlock finding of one validation run. Expired rows
// are retained so the waiver history can always be read back. Only one
// unexpired waiver per (run, finding) may exist; that rule is enforced in the
// service layer under a parent-run lock instead of by a unique constraint,
// because expired history rows share the same finding coordinates.
type ViolationWaiver struct {
	ID              uint          `gorm:"primaryKey"`
	ValidationRunID uint          `gorm:"index:idx_waiver_run,priority:1;not null"`
	FindingKind     string        `gorm:"size:24;index:idx_waiver_finding,priority:1;not null"`
	FindingIndex    int           `gorm:"index:idx_waiver_finding,priority:2;not null"`
	Justification   string        `gorm:"type:text;not null"`
	ExpiresAt       time.Time     `gorm:"index;not null"`
	GrantedBy       uint          `gorm:"index;not null"`
	GrantedByName   string        `gorm:"size:80;not null"`
	GrantedAt       time.Time     `gorm:"index;not null"`
	ValidationRun   ValidationRun `gorm:"foreignKey:ValidationRunID"`
}
