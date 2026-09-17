package algorithm

import (
	"testing"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
)

func TestAnalyzeInterlocks(t *testing.T) {
	tests := []struct {
		name      string
		events    []dto.InterlockEvent
		wantCodes map[string]int
	}{
		{
			name:      "valid directed sequence",
			events:    []dto.InterlockEvent{{Name: "estop", Sequence: 1}, {Name: "gate", Sequence: 2, DependsOn: []string{"estop"}}},
			wantCodes: map[string]int{},
		},
		{
			name:      "missing and reversed",
			events:    []dto.InterlockEvent{{Name: "motion", Sequence: 1, DependsOn: []string{"gate", "missing"}}, {Name: "gate", Sequence: 2}},
			wantCodes: map[string]int{"missing_prerequisite": 1, "reversed_order": 1},
		},
		{
			name:      "cycle",
			events:    []dto.InterlockEvent{{Name: "gate", Sequence: 1, DependsOn: []string{"motion"}}, {Name: "motion", Sequence: 2, DependsOn: []string{"gate"}}},
			wantCodes: map[string]int{"dependency_cycle": 1, "reversed_order": 1},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			counts := map[string]int{}
			for _, finding := range AnalyzeInterlocks(test.events) {
				counts[finding.Code]++
			}
			for code, want := range test.wantCodes {
				if counts[code] != want {
					t.Errorf("code %s count = %d, want %d; all=%v", code, counts[code], want, counts)
				}
			}
			if len(counts) != len(test.wantCodes) {
				t.Errorf("unexpected finding codes: got %v want %v", counts, test.wantCodes)
			}
		})
	}
}

func TestValidateInterlockEvents(t *testing.T) {
	events := []dto.InterlockEvent{{Name: "gate", Sequence: 1}, {Name: "gate", Sequence: 1}}
	if errorsFound := ValidateInterlockEvents(events); len(errorsFound) != 2 {
		t.Fatalf("got %d structural errors, want 2: %v", len(errorsFound), errorsFound)
	}
}
