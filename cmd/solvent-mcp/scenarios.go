package main

// scenario is one named ledger scenario the MCP server exposes. The name is
// what agents pass in tool arguments; the ID is the scenario's fixed UUID in
// the ledger. PipelineFixtures marks scenarios whose evidence flows through
// the generic pipeline fixture loader (solvent_ingest_evidence); those need a
// fixture directory under SOLVENT_FIXTURE_ROOT.
type scenario struct {
	Name             string
	ID               string
	PipelineFixtures bool
}

// scenarios is the single source of truth for scenario names and IDs. Tool
// schema enums, scenario validation, error messages, and fixture-root
// validation are all derived from this list — there is no second copy of the
// name/ID pairs anywhere in the package.
//
// Track 3 is the Agentjacking demo scenario. Its evidence enters through the
// boundary adapter (internal/agentjacking) from the demo's own fixtures, not
// through the generic pipeline loader, so it carries no pipeline fixture
// directory. Adding a scenario here is all that is needed to expose it.
var scenarios = []scenario{
	{"track1", "00000000-0000-0000-0000-000000000001", true},
	{"track2", "00000000-0000-0000-0000-000000000002", true},
	{"track3", "00000000-0000-0000-0000-000000000003", false},
}

// scenarioNames returns the scenario names in declaration order, for use in
// tool input schema enums.
func scenarioNames() []string {
	names := make([]string, len(scenarios))
	for i, s := range scenarios {
		names[i] = s.Name
	}
	return names
}

// lookupScenario maps a scenario name to its fixed UUID.
func lookupScenario(name string) (string, bool) {
	for _, s := range scenarios {
		if s.Name == name {
			return s.ID, true
		}
	}
	return "", false
}
