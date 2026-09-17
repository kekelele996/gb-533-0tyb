package constants

const (
	RoleAdmin           = "admin"
	RoleRobotProgrammer = "robot_programmer"
	RoleSafetyEngineer  = "safety_engineer"
	RoleReviewer        = "reviewer"
	RoleAuditor         = "auditor"

	CellStateDraft    = "draft"
	CellStateFrozen   = "frozen"
	CellStateInactive = "inactive"

	ProgramStateUploaded   = "uploaded"
	ProgramStateParsed     = "parsed"
	ProgramStateReady      = "ready"
	ProgramStateActive     = "active"
	ProgramStateSuperseded = "superseded"
	ProgramStateRejected   = "rejected"

	ValidationQueued     = "queued"
	ValidationSimulating = "simulating"
	ValidationPassed     = "passed"
	ValidationFailed     = "failed"
	ValidationReviewed   = "reviewed"
	ValidationAccepted   = "accepted"
	ValidationVoided     = "voided"
)

func ValidRole(value string) bool {
	switch value {
	case RoleAdmin, RoleRobotProgrammer, RoleSafetyEngineer, RoleReviewer, RoleAuditor:
		return true
	default:
		return false
	}
}

func CanTransitionProgram(from, to string) bool {
	allowed := map[string]map[string]bool{
		ProgramStateUploaded: {ProgramStateParsed: true, ProgramStateRejected: true},
		ProgramStateParsed:   {ProgramStateReady: true, ProgramStateRejected: true},
		ProgramStateReady:    {ProgramStateActive: true, ProgramStateRejected: true},
		ProgramStateActive:   {ProgramStateSuperseded: true},
	}
	return allowed[from][to]
}

func CanTransitionValidation(from, to string) bool {
	allowed := map[string]map[string]bool{
		ValidationQueued:     {ValidationSimulating: true},
		ValidationSimulating: {ValidationPassed: true, ValidationFailed: true},
		ValidationPassed:     {ValidationReviewed: true, ValidationVoided: true},
		ValidationFailed:     {ValidationReviewed: true, ValidationVoided: true},
		ValidationReviewed:   {ValidationAccepted: true, ValidationVoided: true},
		ValidationAccepted:   {ValidationVoided: true},
	}
	return allowed[from][to]
}
