package constants

import "testing"

func TestProgramTransitions(t *testing.T) {
	tests := []struct {
		name      string
		from, to  string
		permitted bool
	}{
		{"upload parses", ProgramStateUploaded, ProgramStateParsed, true},
		{"parsed ready", ProgramStateParsed, ProgramStateReady, true},
		{"ready activates", ProgramStateReady, ProgramStateActive, true},
		{"active superseded", ProgramStateActive, ProgramStateSuperseded, true},
		{"cannot skip parsing", ProgramStateUploaded, ProgramStateActive, false},
		{"superseded terminal", ProgramStateSuperseded, ProgramStateActive, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := CanTransitionProgram(test.from, test.to); actual != test.permitted {
				t.Fatalf("transition %s -> %s = %v, want %v", test.from, test.to, actual, test.permitted)
			}
		})
	}
}

func TestValidationTransitions(t *testing.T) {
	tests := []struct {
		from, to  string
		permitted bool
	}{
		{ValidationQueued, ValidationSimulating, true},
		{ValidationSimulating, ValidationPassed, true},
		{ValidationSimulating, ValidationFailed, true},
		{ValidationPassed, ValidationReviewed, true},
		{ValidationReviewed, ValidationAccepted, true},
		{ValidationAccepted, ValidationVoided, true},
		{ValidationFailed, ValidationAccepted, false},
		{ValidationVoided, ValidationReviewed, false},
	}
	for _, test := range tests {
		if actual := CanTransitionValidation(test.from, test.to); actual != test.permitted {
			t.Errorf("transition %s -> %s = %v, want %v", test.from, test.to, actual, test.permitted)
		}
	}
}
