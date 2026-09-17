package dto

import "time"

type TrajectoryPoint struct {
	XMM      float64 `json:"x_mm"`
	YMM      float64 `json:"y_mm"`
	ZMM      float64 `json:"z_mm"`
	TimeMS   float64 `json:"time_ms"`
	SpeedMMS float64 `json:"speed_mm_s,omitempty"`
}

type InterlockEvent struct {
	Name      string   `json:"name"`
	Sequence  int      `json:"sequence"`
	DependsOn []string `json:"depends_on"`
}

type CreateMotionProgramRequest struct {
	RobotCellID       uint              `json:"robot_cell_id" validate:"required"`
	ProgramCode       string            `json:"program_code" validate:"required,min=3,max=80"`
	Version           int               `json:"version" validate:"required,gte=1,lte=9999"`
	Trajectory        []TrajectoryPoint `json:"trajectory" validate:"required,min=2,dive"`
	ToolRadiusMM      float64           `json:"tool_radius_mm" validate:"gte=0,lte=3000"`
	PayloadRadiusMM   float64           `json:"payload_radius_mm" validate:"gte=0,lte=3000"`
	InterlockSequence []InterlockEvent  `json:"interlock_sequence" validate:"required,min=1,dive"`
}

type ProgramTransitionRequest struct {
	TargetState string `json:"target_state" validate:"required"`
}

type MotionProgramResponse struct {
	ID                uint              `json:"id"`
	RobotCellID       uint              `json:"robot_cell_id"`
	RobotCellCode     string            `json:"robot_cell_code"`
	ProgramCode       string            `json:"program_code"`
	Version           int               `json:"version"`
	Trajectory        []TrajectoryPoint `json:"trajectory"`
	ToolRadiusMM      float64           `json:"tool_radius_mm"`
	PayloadRadiusMM   float64           `json:"payload_radius_mm"`
	InterlockSequence []InterlockEvent  `json:"interlock_sequence"`
	SourceChecksum    string            `json:"source_checksum"`
	ProgramState      string            `json:"program_state"`
	UploadedBy        uint              `json:"uploaded_by"`
	UploadedAt        time.Time         `json:"uploaded_at"`
}

type ProgramParseEvidence struct {
	PointCount     int      `json:"point_count"`
	DurationMS     float64  `json:"duration_ms"`
	InterlockCount int      `json:"interlock_count"`
	SemanticErrors []string `json:"semantic_errors"`
}
