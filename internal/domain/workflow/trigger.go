package workflow

// Trigger represents an event that can cause a state transition
type Trigger string

const (
	TriggerSubmit          Trigger = "SUBMIT"
	TriggerStartAudit      Trigger = "START_AUDIT"
	TriggerCompleteAudit   Trigger = "COMPLETE_AUDIT"
	TriggerRequestReview   Trigger = "REQUEST_REVIEW"
	TriggerAutoApprove     Trigger = "AUTO_APPROVE"
	TriggerApprove         Trigger = "APPROVE"
	TriggerReject          Trigger = "REJECT"
	TriggerStartVoucher    Trigger = "START_VOUCHER"
	TriggerCompleteVoucher Trigger = "COMPLETE_VOUCHER"
	TriggerRetry           Trigger = "RETRY"
)

// String returns the string representation of the trigger
func (t Trigger) String() string {
	return string(t)
}
