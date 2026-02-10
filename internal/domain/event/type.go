package event

// Type identifies the type of domain event
type Type string

const (
	TypeInstanceCreated  Type = "instance.created"
	TypeInstanceApproved Type = "instance.approved"
	TypeInstanceRejected Type = "instance.rejected"
	TypeStatusChanged    Type = "instance.status_changed"
	TypeAttachmentReady  Type = "attachment.ready"
	TypeAuditCompleted   Type = "audit.completed"
	TypeVoucherGenerated Type = "voucher.generated"

	// New event types for task and data pipeline
	TypeTaskCreated       Type = "task.created"
	TypeTaskStatusChanged Type = "task.status_changed"
	TypeDataReady         Type = "instance.data_ready"
	TypeMatchingCompleted Type = "matching.completed"
	TypeInvoiceExtracted  Type = "invoice.extracted"
)

// String returns the string representation of the event type
func (t Type) String() string {
	return string(t)
}

// IsValid checks if the event type is one of the defined constants
func (t Type) IsValid() bool {
	switch t {
	case TypeInstanceCreated,
		TypeInstanceApproved,
		TypeInstanceRejected,
		TypeStatusChanged,
		TypeAttachmentReady,
		TypeAuditCompleted,
		TypeVoucherGenerated,
		TypeTaskCreated,
		TypeTaskStatusChanged,
		TypeDataReady,
		TypeMatchingCompleted,
		TypeInvoiceExtracted:
		return true
	default:
		return false
	}
}
