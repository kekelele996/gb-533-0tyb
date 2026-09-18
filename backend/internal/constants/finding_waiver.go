package constants

// Kinds of validation findings that can carry a reviewer waiver. Informational
// (non-violation) envelope contacts are never waivable because they do not fail
// the run.
const (
	FindingTypeEnvelopeViolation = "envelope_violation"
	FindingTypeInterlockFinding  = "interlock_finding"
)

func ValidFindingType(value string) bool {
	switch value {
	case FindingTypeEnvelopeViolation, FindingTypeInterlockFinding:
		return true
	default:
		return false
	}
}
