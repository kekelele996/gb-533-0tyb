package constants

const (
	ZoneTypeOperating  = "operating"
	ZoneTypeRestricted = "restricted"
	ZoneTypeService    = "service"
	ZoneTypeEscape     = "escape"

	ZoneStateDraft    = "draft"
	ZoneStateActive   = "active"
	ZoneStateInactive = "inactive"
)

func ValidZoneType(value string) bool {
	switch value {
	case ZoneTypeOperating, ZoneTypeRestricted, ZoneTypeService, ZoneTypeEscape:
		return true
	default:
		return false
	}
}

func ValidZoneState(value string) bool {
	switch value {
	case ZoneStateDraft, ZoneStateActive, ZoneStateInactive:
		return true
	default:
		return false
	}
}
