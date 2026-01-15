package workflow

import (
	"testing"

	"github.com/garyjia/ai-reimbursement/internal/models"
	"go.uber.org/zap"
)

func TestMapLarkStatus(t *testing.T) {
	tests := []struct {
		name       string
		larkStatus string
		want       string
	}{
		{
			name:       "pending status",
			larkStatus: "PENDING",
			want:       models.StatusPending,
		},
		{
			name:       "approved status",
			larkStatus: "APPROVED",
			want:       models.StatusApproved,
		},
		{
			name:       "rejected status",
			larkStatus: "REJECTED",
			want:       models.StatusRejected,
		},
		{
			name:       "unknown status defaults to pending",
			larkStatus: "UNKNOWN",
			want:       models.StatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapLarkStatus(tt.larkStatus)
			if got != tt.want {
				t.Errorf("mapLarkStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatusTransitions(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tracker := &StatusTracker{logger: logger}

	tests := []struct {
		name      string
		fromStatus string
		toStatus   string
		wantValid  bool
	}{
		{
			name:       "valid transition from CREATED to PENDING",
			fromStatus: models.StatusCreated,
			toStatus:   models.StatusPending,
			wantValid:  true,
		},
		{
			name:       "valid transition from PENDING to AI_AUDITING",
			fromStatus: models.StatusPending,
			toStatus:   models.StatusAIAuditing,
			wantValid:  true,
		},
		{
			name:       "invalid transition from COMPLETED to PENDING",
			fromStatus: models.StatusCompleted,
			toStatus:   models.StatusPending,
			wantValid:  false,
		},
		{
			name:       "idempotent transition (same status)",
			fromStatus: models.StatusPending,
			toStatus:   models.StatusPending,
			wantValid:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tracker.isValidTransition(tt.fromStatus, tt.toStatus)
			if got != tt.wantValid {
				t.Errorf("isValidTransition() = %v, want %v", got, tt.wantValid)
			}
		})
	}
}
