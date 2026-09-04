package policy

import (
	"testing"
)

func TestClassWithinLimits(t *testing.T) {
	tests := []struct {
		name     string
		tool     ToolClass
		max      ToolClass
		expected bool
	}{
		{"read within read", ClassRead, ClassRead, true},
		{"read within mutate", ClassRead, ClassMutate, true},
		{"read within orchestrate", ClassRead, ClassOrchestrate, true},
		{"mutate within read", ClassMutate, ClassRead, false},
		{"mutate within mutate", ClassMutate, ClassMutate, true},
		{"mutate within orchestrate", ClassMutate, ClassOrchestrate, true},
		{"orchestrate within read", ClassOrchestrate, ClassRead, false},
		{"orchestrate within mutate", ClassOrchestrate, ClassMutate, false},
		{"orchestrate within orchestrate", ClassOrchestrate, ClassOrchestrate, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classWithinLimits(tt.tool, tt.max)
			if result != tt.expected {
				t.Errorf("classWithinLimits(%s, %s) = %v, want %v", tt.tool, tt.max, result, tt.expected)
			}
		})
	}
}

func TestMissingBeliefs(t *testing.T) {
	tests := []struct {
		name     string
		required []string
		promoted []string
		expected []string
	}{
		{
			"all present",
			[]string{"belief_a", "belief_b"},
			[]string{"belief_a", "belief_b"},
			nil,
		},
		{
			"some missing",
			[]string{"belief_a", "belief_b", "belief_c"},
			[]string{"belief_a"},
			[]string{"belief_b", "belief_c"},
		},
		{
			"none required",
			nil,
			[]string{"belief_a"},
			nil,
		},
		{
			"all missing",
			[]string{"x", "y"},
			nil,
			[]string{"x", "y"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := missingBeliefs(tt.required, tt.promoted)
			if len(result) != len(tt.expected) {
				t.Errorf("missingBeliefs returned %v, want %v", result, tt.expected)
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("missingBeliefs[%d] = %q, want %q", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

func TestPolicyConstraintsIsSatisfied(t *testing.T) {
	tests := []struct {
		name     string
		pc       PolicyConstraints
		expected bool
	}{
		{
			"all satisfied",
			PolicyConstraints{ActorPermitted: true, StagePermitted: true, ToolPermitted: true},
			true,
		},
		{
			"actor denied",
			PolicyConstraints{ActorPermitted: false, StagePermitted: true, ToolPermitted: true},
			false,
		},
		{
			"tool denied",
			PolicyConstraints{ActorPermitted: true, StagePermitted: true, ToolPermitted: false},
			false,
		},
		{
			"all denied",
			PolicyConstraints{ActorPermitted: false, StagePermitted: false, ToolPermitted: false},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.pc.IsSatisfied()
			if result != tt.expected {
				t.Errorf("IsSatisfied() = %v, want %v", result, tt.expected)
			}
		})
	}
}
