package workflow

import (
	domainwf "github.com/garyjia/ai-reimbursement/internal/domain/workflow"
)

// BuildReimbursementStateMachine creates a state machine configured for reimbursement workflow
func BuildReimbursementStateMachine(initialState domainwf.State) domainwf.StateMachine {
	builder := domainwf.NewBuilder()

	// CREATED state transitions
	builder.Configure(domainwf.StateCreated).
		Permit(domainwf.TriggerSubmit, domainwf.StatePending).
		Permit(domainwf.TriggerStartAudit, domainwf.StateAIAuditing)

	// PENDING state transitions
	builder.Configure(domainwf.StatePending).
		Permit(domainwf.TriggerStartAudit, domainwf.StateAIAuditing).
		Permit(domainwf.TriggerReject, domainwf.StateRejected)

	// AI_AUDITING state transitions
	builder.Configure(domainwf.StateAIAuditing).
		Permit(domainwf.TriggerCompleteAudit, domainwf.StateAIAudited).
		Permit(domainwf.TriggerReject, domainwf.StateRejected)

	// AI_AUDITED state transitions
	builder.Configure(domainwf.StateAIAudited).
		Permit(domainwf.TriggerAutoApprove, domainwf.StateAutoApproved).
		Permit(domainwf.TriggerRequestReview, domainwf.StateInReview).
		Permit(domainwf.TriggerReject, domainwf.StateRejected)

	// IN_REVIEW state transitions
	builder.Configure(domainwf.StateInReview).
		Permit(domainwf.TriggerApprove, domainwf.StateApproved).
		Permit(domainwf.TriggerReject, domainwf.StateRejected)

	// AUTO_APPROVED state transitions
	builder.Configure(domainwf.StateAutoApproved).
		Permit(domainwf.TriggerApprove, domainwf.StateApproved).
		Permit(domainwf.TriggerReject, domainwf.StateRejected)

	// APPROVED state transitions
	builder.Configure(domainwf.StateApproved).
		Permit(domainwf.TriggerStartVoucher, domainwf.StateVoucherGenerating)

	// VOUCHER_GENERATING state transitions
	builder.Configure(domainwf.StateVoucherGenerating).
		Permit(domainwf.TriggerCompleteVoucher, domainwf.StateCompleted).
		Permit(domainwf.TriggerRetry, domainwf.StateApproved)

	// REJECTED and COMPLETED are terminal states - no outgoing transitions

	return builder.Build(initialState)
}
