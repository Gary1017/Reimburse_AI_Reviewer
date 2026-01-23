package workflow

// State represents a workflow state in the approval lifecycle
type State string

const (
	StateCreated           State = "CREATED"
	StatePending           State = "PENDING"
	StateAIAuditing        State = "AI_AUDITING"
	StateAIAudited         State = "AI_AUDITED"
	StateInReview          State = "IN_REVIEW"
	StateAutoApproved      State = "AUTO_APPROVED"
	StateApproved          State = "APPROVED"
	StateRejected          State = "REJECTED"
	StateVoucherGenerating State = "VOUCHER_GENERATING"
	StateCompleted         State = "COMPLETED"
)

var validStates = map[State]bool{
	StateCreated:           true,
	StatePending:           true,
	StateAIAuditing:        true,
	StateAIAudited:         true,
	StateInReview:          true,
	StateAutoApproved:      true,
	StateApproved:          true,
	StateRejected:          true,
	StateVoucherGenerating: true,
	StateCompleted:         true,
}

var terminalStates = map[State]bool{
	StateRejected:  true,
	StateCompleted: true,
}

// IsTerminal returns true if the state is a terminal state (no further transitions allowed)
func (s State) IsTerminal() bool {
	return terminalStates[s]
}

// String returns the string representation of the state
func (s State) String() string {
	return string(s)
}

// IsValid returns true if the state is a valid workflow state
func (s State) IsValid() bool {
	return validStates[s]
}
