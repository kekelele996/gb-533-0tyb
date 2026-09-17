package algorithm

import (
	"fmt"
	"sort"
	"strings"

	"robot-cell-safety-envelope-validator/backend/internal/dto"
)

func ValidateInterlockEvents(events []dto.InterlockEvent) []string {
	errorsFound := make([]string, 0)
	if len(events) == 0 {
		return []string{"interlock sequence must contain at least one event"}
	}
	seenNames := map[string]bool{}
	seenSequences := map[int]string{}
	for index, event := range events {
		name := strings.TrimSpace(event.Name)
		if name == "" {
			errorsFound = append(errorsFound, fmt.Sprintf("event %d has no name", index))
		}
		if event.Sequence < 1 {
			errorsFound = append(errorsFound, fmt.Sprintf("event %q sequence must be positive", name))
		}
		if seenNames[name] {
			errorsFound = append(errorsFound, fmt.Sprintf("event name %q is duplicated", name))
		}
		if previous, exists := seenSequences[event.Sequence]; exists {
			errorsFound = append(errorsFound, fmt.Sprintf("sequence %d is shared by %q and %q", event.Sequence, previous, name))
		}
		seenNames[name] = true
		seenSequences[event.Sequence] = name
	}
	return errorsFound
}

func AnalyzeInterlocks(events []dto.InterlockEvent) []dto.InterlockFinding {
	findings := make([]dto.InterlockFinding, 0)
	byName := make(map[string]dto.InterlockEvent, len(events))
	for _, event := range events {
		byName[event.Name] = event
	}
	for _, event := range events {
		for _, dependency := range event.DependsOn {
			prerequisite, exists := byName[dependency]
			if !exists {
				findings = append(findings, dto.InterlockFinding{
					Code: "missing_prerequisite", Event: event.Name, DependsOn: dependency,
					Evidence: fmt.Sprintf("%s declares %s as a prerequisite, but no such event exists", event.Name, dependency),
				})
				continue
			}
			if prerequisite.Sequence >= event.Sequence {
				findings = append(findings, dto.InterlockFinding{
					Code: "reversed_order", Event: event.Name, DependsOn: dependency,
					Evidence: fmt.Sprintf("%s is sequence %d but prerequisite %s is sequence %d", event.Name, event.Sequence, dependency, prerequisite.Sequence),
				})
			}
		}
	}
	for _, path := range dependencyCycles(events, byName) {
		findings = append(findings, dto.InterlockFinding{
			Code: "dependency_cycle", Event: path[0], Path: path,
			Evidence: "directed prerequisite cycle: " + strings.Join(path, " -> "),
		})
	}
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Code == findings[j].Code {
			return findings[i].Event < findings[j].Event
		}
		return findings[i].Code < findings[j].Code
	})
	return findings
}

func dependencyCycles(events []dto.InterlockEvent, byName map[string]dto.InterlockEvent) [][]string {
	state := map[string]int{}
	stack := make([]string, 0, len(events))
	cycles := make([][]string, 0)
	seenCycle := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if state[name] == 2 {
			return
		}
		if state[name] == 1 {
			start := 0
			for index, item := range stack {
				if item == name {
					start = index
					break
				}
			}
			cycle := append(append([]string{}, stack[start:]...), name)
			key := canonicalCycleKey(cycle)
			if !seenCycle[key] {
				seenCycle[key] = true
				cycles = append(cycles, cycle)
			}
			return
		}
		state[name] = 1
		stack = append(stack, name)
		for _, dependency := range byName[name].DependsOn {
			if _, exists := byName[dependency]; exists {
				visit(dependency)
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = 2
	}
	for _, event := range events {
		visit(event.Name)
	}
	return cycles
}

func canonicalCycleKey(path []string) string {
	if len(path) <= 1 {
		return strings.Join(path, "|")
	}
	items := append([]string{}, path[:len(path)-1]...)
	sort.Strings(items)
	return strings.Join(items, "|")
}
