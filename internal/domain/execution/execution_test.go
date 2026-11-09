package execution

import (
	"testing"
	"time"
)

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name string
		from Status
		to   Status
		want bool
	}{
		{name: "pending to queued", from: StatusPending, to: StatusQueued, want: true},
		{name: "pending to cancelled", from: StatusPending, to: StatusCancelled, want: true},
		{name: "pending to running invalid", from: StatusPending, to: StatusRunning, want: false},
		{name: "queued to running", from: StatusQueued, to: StatusRunning, want: true},
		{name: "queued to completed invalid", from: StatusQueued, to: StatusCompleted, want: false},
		{name: "running to completed", from: StatusRunning, to: StatusCompleted, want: true},
		{name: "running to retrying", from: StatusRunning, to: StatusRetrying, want: true},
		{name: "running to failed", from: StatusRunning, to: StatusFailed, want: true},
		{name: "running to dead_letter", from: StatusRunning, to: StatusDeadLetter, want: true},
		{name: "running to queued (lease expired)", from: StatusRunning, to: StatusQueued, want: true},
		{name: "running to cancelled", from: StatusRunning, to: StatusCancelled, want: true},
		{name: "running to pending invalid", from: StatusRunning, to: StatusPending, want: false},
		{name: "retrying to queued", from: StatusRetrying, to: StatusQueued, want: true},
		{name: "retrying to dead_letter", from: StatusRetrying, to: StatusDeadLetter, want: true},
		{name: "retrying to cancelled", from: StatusRetrying, to: StatusCancelled, want: true},
		{name: "retrying to completed invalid", from: StatusRetrying, to: StatusCompleted, want: false},
		{name: "dead_letter to queued (redrive)", from: StatusDeadLetter, to: StatusQueued, want: true},
		{name: "dead_letter to failed invalid", from: StatusDeadLetter, to: StatusFailed, want: false},
		{name: "completed terminal", from: StatusCompleted, to: StatusQueued, want: false},
		{name: "failed terminal", from: StatusFailed, to: StatusQueued, want: false},
		{name: "cancelled terminal", from: StatusCancelled, to: StatusQueued, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanTransition(tt.from, tt.to); got != tt.want {
				t.Fatalf("CanTransition(%s,%s)=%v want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestExecutionValidatesteInvariants(t *testing.T) {
	now := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		e       Execution
		wantErr bool
	}{
		{
			name: "ok",
			e: Execution{
				ID: "e1", TaskID: "t1", Status: StatusPending,
				Attempt: 0, MaxAttempts: 1, ScheduledFor: now,
			},
			wantErr: false,
		},
		{name: "missing task_id", e: Execution{Status: StatusPending, MaxAttempts: 1, ScheduledFor: now}, wantErr: true},
		{name: "missing scheduled_for", e: Execution{TaskID: "t1", Status: StatusPending, MaxAttempts: 1}, wantErr: true},
		{name: "invalid max_attempts", e: Execution{TaskID: "t1", Status: StatusPending, MaxAttempts: 0, ScheduledFor: now}, wantErr: true},
		{name: "negative attempt", e: Execution{TaskID: "t1", Status: StatusPending, Attempt: -1, MaxAttempts: 1, ScheduledFor: now}, wantErr: true},
		{name: "invalid status", e: Execution{TaskID: "t1", Status: "bogus", MaxAttempts: 1, ScheduledFor: now}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.e.ValidatesteInvariants()
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatesteInvariants() err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}
