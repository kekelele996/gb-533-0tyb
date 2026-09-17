package geometry

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
)

const epsilon = 1e-9

type Point2D struct {
	X float64
	Y float64
}

type Polygon struct {
	Ring []Point2D
}

type ZoneVolume struct {
	ID            uint
	Name          string
	ZoneType      string
	Polygon       Polygon
	MinHeightMM   float64
	MaxHeightMM   float64
	SpeedLimitMMS float64
}

type geoJSONDocument struct {
	Type        string          `json:"type"`
	Coordinates [][][]float64   `json:"coordinates"`
	Geometry    json.RawMessage `json:"geometry"`
}

func ParsePolygon(raw []byte) (Polygon, error) {
	var document geoJSONDocument
	if err := json.Unmarshal(raw, &document); err != nil {
		return Polygon{}, fmt.Errorf("decode GeoJSON: %w", err)
	}
	if document.Type == "Feature" {
		if len(document.Geometry) == 0 {
			return Polygon{}, errors.New("GeoJSON feature has no geometry")
		}
		return ParsePolygon(document.Geometry)
	}
	if document.Type != "Polygon" {
		return Polygon{}, fmt.Errorf("geometry type %q is not Polygon", document.Type)
	}
	if len(document.Coordinates) != 1 || len(document.Coordinates[0]) < 4 {
		return Polygon{}, errors.New("polygon must contain one exterior ring with at least four coordinates")
	}
	ring := make([]Point2D, 0, len(document.Coordinates[0]))
	for index, coordinate := range document.Coordinates[0] {
		if len(coordinate) < 2 || !finite(coordinate[0]) || !finite(coordinate[1]) {
			return Polygon{}, fmt.Errorf("coordinate %d must contain finite x/y values", index)
		}
		ring = append(ring, Point2D{X: coordinate[0], Y: coordinate[1]})
	}
	if !samePoint(ring[0], ring[len(ring)-1]) {
		return Polygon{}, errors.New("polygon exterior ring must be closed")
	}
	polygon := Polygon{Ring: ring}
	if math.Abs(polygonArea(polygon)) < 1 {
		return Polygon{}, errors.New("polygon area must be at least one square millimetre")
	}
	if selfIntersects(polygon) {
		return Polygon{}, errors.New("polygon exterior ring self-intersects")
	}
	return polygon, nil
}

func ValidateTrajectory(points []dto.TrajectoryPoint) error {
	if len(points) < 2 {
		return errors.New("trajectory must contain at least two points")
	}
	for index, point := range points {
		if !finite(point.XMM) || !finite(point.YMM) || !finite(point.ZMM) || !finite(point.TimeMS) || !finite(point.SpeedMMS) {
			return fmt.Errorf("trajectory point %d contains a non-finite value", index)
		}
		if point.ZMM < 0 || point.TimeMS < 0 || point.SpeedMMS < 0 {
			return fmt.Errorf("trajectory point %d contains a negative height, time, or speed", index)
		}
		if index > 0 && point.TimeMS <= points[index-1].TimeMS {
			return fmt.Errorf("trajectory point %d time must increase strictly", index)
		}
	}
	return nil
}

func EvaluateEnvelope(points []dto.TrajectoryPoint, expansionRadiusMM float64, zones []ZoneVolume) []dto.CollisionEvent {
	events := make([]dto.CollisionEvent, 0)
	for segmentIndex := 0; segmentIndex < len(points)-1; segmentIndex++ {
		from, to := points[segmentIndex], points[segmentIndex+1]
		actualSpeed := segmentSpeed(from, to)
		for _, zone := range zones {
			fraction, clearance, intersects := firstIntersection(from, to, expansionRadiusMM, zone)
			if !intersects {
				continue
			}
			tooFast := zone.SpeedLimitMMS > 0 && actualSpeed > zone.SpeedLimitMMS+epsilon
			violatesBoundary := zone.ZoneType != "operating"
			events = append(events, dto.CollisionEvent{
				SegmentIndex: segmentIndex,
				FirstTimeMS:  from.TimeMS + fraction*(to.TimeMS-from.TimeMS),
				ZoneID:       zone.ID, ZoneName: zone.Name, ZoneType: zone.ZoneType,
				AllowedSpeedMMS: zone.SpeedLimitMMS, ActualSpeedMMS: actualSpeed,
				ClearanceMM: clearance, Violation: tooFast || violatesBoundary,
				Evidence: envelopeEvidence(zone, expansionRadiusMM, tooFast, violatesBoundary),
			})
		}
	}
	return events
}

func firstIntersection(from, to dto.TrajectoryPoint, radius float64, zone ZoneVolume) (float64, float64, bool) {
	bestClearance := math.Inf(1)
	for step := 0; step <= 100; step++ {
		fraction := float64(step) / 100
		x := from.XMM + fraction*(to.XMM-from.XMM)
		y := from.YMM + fraction*(to.YMM-from.YMM)
		z := from.ZMM + fraction*(to.ZMM-from.ZMM)
		if z+radius < zone.MinHeightMM || z-radius > zone.MaxHeightMM {
			continue
		}
		distance := signedDistance(Point2D{X: x, Y: y}, zone.Polygon)
		clearance := distance - radius
		if clearance < bestClearance {
			bestClearance = clearance
		}
		if clearance <= epsilon {
			return fraction, clearance, true
		}
	}
	return 0, bestClearance, false
}

func segmentSpeed(from, to dto.TrajectoryPoint) float64 {
	if to.SpeedMMS > 0 {
		return to.SpeedMMS
	}
	durationSeconds := (to.TimeMS - from.TimeMS) / 1000
	if durationSeconds <= 0 {
		return math.Inf(1)
	}
	dx, dy, dz := to.XMM-from.XMM, to.YMM-from.YMM, to.ZMM-from.ZMM
	return math.Sqrt(dx*dx+dy*dy+dz*dz) / durationSeconds
}

func envelopeEvidence(zone ZoneVolume, radius float64, tooFast, boundary bool) string {
	reason := "expanded envelope intersects the configured zone volume"
	if boundary && tooFast {
		reason += "; zone access boundary and speed limit are both exceeded"
	} else if boundary {
		reason += "; zone type requires independent safety review"
	} else if tooFast {
		reason += "; actual speed exceeds the configured zone limit"
	} else {
		reason += "; operating-zone contact is informational"
	}
	return fmt.Sprintf("%s (2D radius %.1f mm, height interval %.1f..%.1f mm)", reason, radius, zone.MinHeightMM, zone.MaxHeightMM)
}

func signedDistance(point Point2D, polygon Polygon) float64 {
	minimum := math.Inf(1)
	for index := 0; index < len(polygon.Ring)-1; index++ {
		distance := pointSegmentDistance(point, polygon.Ring[index], polygon.Ring[index+1])
		if distance < minimum {
			minimum = distance
		}
	}
	if pointInPolygon(point, polygon) {
		return -minimum
	}
	return minimum
}

func pointInPolygon(point Point2D, polygon Polygon) bool {
	inside := false
	for i, j := 0, len(polygon.Ring)-2; i < len(polygon.Ring)-1; j, i = i, i+1 {
		left, right := polygon.Ring[i], polygon.Ring[j]
		crosses := (left.Y > point.Y) != (right.Y > point.Y)
		if crosses && point.X < (right.X-left.X)*(point.Y-left.Y)/(right.Y-left.Y)+left.X {
			inside = !inside
		}
	}
	return inside
}

func pointSegmentDistance(point, start, end Point2D) float64 {
	dx, dy := end.X-start.X, end.Y-start.Y
	lengthSquared := dx*dx + dy*dy
	if lengthSquared <= epsilon {
		return math.Hypot(point.X-start.X, point.Y-start.Y)
	}
	projection := ((point.X-start.X)*dx + (point.Y-start.Y)*dy) / lengthSquared
	projection = math.Max(0, math.Min(1, projection))
	closest := Point2D{X: start.X + projection*dx, Y: start.Y + projection*dy}
	return math.Hypot(point.X-closest.X, point.Y-closest.Y)
}

func polygonArea(polygon Polygon) float64 {
	area := 0.0
	for index := 0; index < len(polygon.Ring)-1; index++ {
		left, right := polygon.Ring[index], polygon.Ring[index+1]
		area += left.X*right.Y - right.X*left.Y
	}
	return area / 2
}

func selfIntersects(polygon Polygon) bool {
	edges := len(polygon.Ring) - 1
	for first := 0; first < edges; first++ {
		for second := first + 1; second < edges; second++ {
			if second == first+1 || (first == 0 && second == edges-1) {
				continue
			}
			if segmentsIntersect(polygon.Ring[first], polygon.Ring[first+1], polygon.Ring[second], polygon.Ring[second+1]) {
				return true
			}
		}
	}
	return false
}

func segmentsIntersect(a, b, c, d Point2D) bool {
	orientation := func(p, q, r Point2D) float64 {
		return (q.Y-p.Y)*(r.X-q.X) - (q.X-p.X)*(r.Y-q.Y)
	}
	o1, o2 := orientation(a, b, c), orientation(a, b, d)
	o3, o4 := orientation(c, d, a), orientation(c, d, b)
	return o1*o2 < -epsilon && o3*o4 < -epsilon
}

func samePoint(left, right Point2D) bool {
	return math.Abs(left.X-right.X) <= epsilon && math.Abs(left.Y-right.Y) <= epsilon
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
