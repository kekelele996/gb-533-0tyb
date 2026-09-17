package dto

import (
	"encoding/json"
	"time"
)

type CreateSafetyZoneRequest struct {
	RobotCellID    uint            `json:"robot_cell_id" validate:"required"`
	Name           string          `json:"name" validate:"required,min=2,max=120"`
	ZoneType       string          `json:"zone_type" validate:"required"`
	PolygonGeoJSON json.RawMessage `json:"polygon_geojson" validate:"required"`
	MinHeightMM    float64         `json:"min_height_mm" validate:"gte=0,lte=30000"`
	MaxHeightMM    float64         `json:"max_height_mm" validate:"required,gt=0,lte=30000"`
	SpeedLimitMMS  float64         `json:"speed_limit_mm_s" validate:"gte=0,lte=10000"`
	AccessRule     string          `json:"access_rule" validate:"required,max=300"`
}

type UpdateSafetyZoneRequest struct {
	Name           string          `json:"name" validate:"required,min=2,max=120"`
	ZoneType       string          `json:"zone_type" validate:"required"`
	PolygonGeoJSON json.RawMessage `json:"polygon_geojson" validate:"required"`
	MinHeightMM    float64         `json:"min_height_mm" validate:"gte=0,lte=30000"`
	MaxHeightMM    float64         `json:"max_height_mm" validate:"required,gt=0,lte=30000"`
	SpeedLimitMMS  float64         `json:"speed_limit_mm_s" validate:"gte=0,lte=10000"`
	AccessRule     string          `json:"access_rule" validate:"required,max=300"`
	Version        int             `json:"version" validate:"required,gte=1"`
}

type SafetyZoneResponse struct {
	ID             uint            `json:"id"`
	RobotCellID    uint            `json:"robot_cell_id"`
	RobotCellCode  string          `json:"robot_cell_code"`
	Name           string          `json:"name"`
	ZoneType       string          `json:"zone_type"`
	PolygonGeoJSON json.RawMessage `json:"polygon_geojson"`
	MinHeightMM    float64         `json:"min_height_mm"`
	MaxHeightMM    float64         `json:"max_height_mm"`
	SpeedLimitMMS  float64         `json:"speed_limit_mm_s"`
	AccessRule     string          `json:"access_rule"`
	ZoneState      string          `json:"zone_state"`
	Version        int             `json:"version"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}
