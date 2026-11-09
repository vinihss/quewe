package task

import (
	"strings"
	"testing"
	"time"
)

func TestValidatesteName(t *testing.T) {
	valid64 := "a" + strings.Repeat("b", 62) + "c"
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "valid simple", input: "abc", wantErr: false},
		{name: "valid with hyphen", input: "daily-arch-update", wantErr: false},
		{name: "valid min length", input: "a1b", wantErr: false},
		{name: "valid 64 chars", input: valid64, wantErr: false},
		{name: "too short", input: "ab", wantErr: true},
		{name: "too long", input: valid64 + "d", wantErr: true},
		{name: "uppercase", input: "Daily", wantErr: true},
		{name: "starts with hyphen", input: "-abc", wantErr: true},
		{name: "ends with hyphen", input: "abc-", wantErr: true},
		{name: "underscore", input: "ab_c", wantErr: true},
		{name: "space", input: "ab c", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatesteName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidatesteName(%q) err=%v wantErr=%v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestExecutionPolicyValidateste(t *testing.T) {
	tests := []struct {
		name    string
		p       ExecutionPolicy
		wantErr bool
	}{
		{name: "ok", p: ExecutionPolicy{TimeoutSeconds: 30, MaxRetries: 0, Priority: 5}, wantErr: false},
		{name: "priority low", p: ExecutionPolicy{TimeoutSeconds: 1, MaxRetries: 0, Priority: 0}, wantErr: false},
		{name: "priority high", p: ExecutionPolicy{TimeoutSeconds: 1, MaxRetries: 0, Priority: 9}, wantErr: false},
		{name: "priority negative", p: ExecutionPolicy{TimeoutSeconds: 1, MaxRetries: 0, Priority: -1}, wantErr: true},
		{name: "priority too high", p: ExecutionPolicy{TimeoutSeconds: 1, MaxRetries: 0, Priority: 10}, wantErr: true},
		{name: "timeout zero", p: ExecutionPolicy{TimeoutSeconds: 0, MaxRetries: 0, Priority: 5}, wantErr: true},
		{name: "timeout negative", p: ExecutionPolicy{TimeoutSeconds: -1, MaxRetries: 0, Priority: 5}, wantErr: true},
		{name: "max retries negative", p: ExecutionPolicy{TimeoutSeconds: 1, MaxRetries: -1, Priority: 5}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.p.Validateste()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validateste() err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
	if got := (ExecutionPolicy{MaxRetries: 2}).MaxAttempts(); got != 3 {
		t.Fatalf("MaxAttempts=%d want 3", got)
	}
}

func TestScheduleValidateste(t *testing.T) {
	runAt := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		s       Schedule
		wantErr bool
	}{
		{name: "manual ok", s: Schedule{Type: ScheduleManual}, wantErr: false},
		{name: "once ok", s: Schedule{Type: ScheduleOnce, RunAt: &runAt}, wantErr: false},
		{name: "once missing run_at", s: Schedule{Type: ScheduleOnce}, wantErr: true},
		{name: "cron ok", s: Schedule{Type: ScheduleCron, Expression: "0 4 * * *", Timezone: "America/Sao_Paulo"}, wantErr: false},
		{name: "cron missing expr", s: Schedule{Type: ScheduleCron, Timezone: "UTC"}, wantErr: true},
		{name: "cron missing tz", s: Schedule{Type: ScheduleCron, Expression: "0 4 * * *"}, wantErr: true},
		{name: "unknown type", s: Schedule{Type: "weekly"}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.s.Validateste()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validateste() err=%v wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestTargetValidateste(t *testing.T) {
	if err := (Target{Capability: "system.maintenance"}).Validateste(); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if err := (Target{}).Validateste(); err == nil {
		t.Fatal("expected error for empty capability")
	}
}
