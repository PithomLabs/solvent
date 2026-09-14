package belief

// Named constants for the deployment-review debt vocabulary.
// These are domain-specific obligation identifiers, not Solvent kernel concepts.
// Both belief creation (starting debt) and evidence mapping (retirement rules)
// reference this single source of truth.
//
// Do NOT duplicate these strings elsewhere. If a new domain needs its own
// vocabulary, define it in that domain's package — not here.
const (
	NeedProvenanceCheck    = "needProvenanceCheck"
	NeedContradictionSweep = "needContradictionSweep"
	NeedBlastRadius        = "needBlastRadius"
	NeedRollbackPlan       = "needRollbackPlan"
	NeedVersionPin         = "needVersionPin"
	NeedOperatorSignoff    = "needOperatorSignoff"
)

// wizardDebt is the deployment-review starting debt.
// It is package-internal; external callers must use WizardDebt().
var wizardDebt = []string{
	NeedProvenanceCheck,
	NeedContradictionSweep,
	NeedBlastRadius,
	NeedRollbackPlan,
	NeedVersionPin,
	NeedOperatorSignoff,
}

// WizardDebt returns the deployment-review starting debt vocabulary.
// Returns a defensive copy to prevent mutation of the backing slice.
func WizardDebt() []string {
	out := make([]string, len(wizardDebt))
	copy(out, wizardDebt)
	return out
}
