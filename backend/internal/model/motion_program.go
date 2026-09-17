package model

import "time"

type MotionProgram struct {
	ID                    uint      `gorm:"primaryKey"`
	RobotCellID           uint      `gorm:"index;not null"`
	ProgramCode           string    `gorm:"size:80;index:idx_program_version,unique;not null"`
	Version               int       `gorm:"index:idx_program_version,unique;not null"`
	TrajectoryJSON        string    `gorm:"type:text;not null"`
	ToolRadiusMM          float64   `gorm:"not null"`
	PayloadRadiusMM       float64   `gorm:"not null"`
	InterlockSequenceJSON string    `gorm:"type:text;not null"`
	SourceChecksum        string    `gorm:"size:64;index;not null"`
	ProgramState          string    `gorm:"size:24;index;not null"`
	UploadedBy            uint      `gorm:"index;not null"`
	UploadedAt            time.Time `gorm:"index;not null"`
	UpdatedAt             time.Time `gorm:"not null"`
	RobotCell             RobotCell `gorm:"foreignKey:RobotCellID"`
}
