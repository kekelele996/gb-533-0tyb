package geometry

import (
	"testing"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
)

func TestParsePolygonValidation(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"valid square", `{"type":"Polygon","coordinates":[[[0,0],[100,0],[100,100],[0,100],[0,0]]]}`, false},
		{"feature wrapper", `{"type":"Feature","geometry":{"type":"Polygon","coordinates":[[[0,0],[100,0],[100,100],[0,100],[0,0]]]},"properties":{}}`, false},
		{"not closed", `{"type":"Polygon","coordinates":[[[0,0],[100,0],[100,100],[0,100]]]}`, true},
		{"self intersecting", `{"type":"Polygon","coordinates":[[[0,0],[100,100],[0,100],[100,0],[0,0]]]}`, true},
		{"wrong geometry", `{"type":"LineString","coordinates":[[0,0],[1,1]]}`, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := ParsePolygon([]byte(test.raw))
			if (err != nil) != test.wantErr {
				t.Fatalf("ParsePolygon() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestEvaluateEnvelope(t *testing.T) {
	polygon, err := ParsePolygon([]byte(`{"type":"Polygon","coordinates":[[[100,-50],[200,-50],[200,50],[100,50],[100,-50]]]}`))
	if err != nil {
		t.Fatal(err)
	}
	zones := []ZoneVolume{{ID: 7, Name: "gate", ZoneType: "restricted", Polygon: polygon, MinHeightMM: 0, MaxHeightMM: 1000, SpeedLimitMMS: 20}}
	tests := []struct {
		name       string
		points     []dto.TrajectoryPoint
		radius     float64
		wantEvents int
	}{
		{"direct crossing", []dto.TrajectoryPoint{{XMM: 0, YMM: 0, ZMM: 500, TimeMS: 0}, {XMM: 300, YMM: 0, ZMM: 500, TimeMS: 1000}}, 10, 1},
		{"radius expansion", []dto.TrajectoryPoint{{XMM: 0, YMM: 70, ZMM: 500, TimeMS: 0}, {XMM: 300, YMM: 70, ZMM: 500, TimeMS: 1000}}, 25, 1},
		{"outside height", []dto.TrajectoryPoint{{XMM: 0, YMM: 0, ZMM: 1500, TimeMS: 0}, {XMM: 300, YMM: 0, ZMM: 1500, TimeMS: 1000}}, 10, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			events := EvaluateEnvelope(test.points, test.radius, zones)
			if len(events) != test.wantEvents {
				t.Fatalf("got %d events, want %d", len(events), test.wantEvents)
			}
			if len(events) > 0 && (!events[0].Violation || events[0].FirstTimeMS < 0) {
				t.Fatalf("unexpected event evidence: %+v", events[0])
			}
		})
	}
}

func TestValidateTrajectoryTimeAndFiniteValues(t *testing.T) {
	valid := []dto.TrajectoryPoint{{TimeMS: 0}, {XMM: 1, TimeMS: 1}}
	if err := ValidateTrajectory(valid); err != nil {
		t.Fatalf("valid trajectory rejected: %v", err)
	}
	invalid := []dto.TrajectoryPoint{{TimeMS: 1}, {XMM: 1, TimeMS: 1}}
	if err := ValidateTrajectory(invalid); err == nil {
		t.Fatal("non-increasing time was accepted")
	}
}
