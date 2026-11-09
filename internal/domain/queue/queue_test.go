package queue

import (
	"testing"
	"time"
)

func TestCanTransition(t *testing.T) {
	cases := []struct {
		from, to Status
		ok       bool
	}{
		{StatusQueued, StatusReserved, true},
		{StatusReserved, StatusQueued, true},
		{StatusReserved, StatusCompleted, true},
		{StatusReserved, StatusDeadLetter, true},
		{StatusQueued, StatusCompleted, false},
		{StatusCompleted, StatusQueued, false},
	}
	for _, c := range cases {
		if got := CanTransition(c.from, c.to); got != c.ok {
			t.Errorf("%s->%s: got %v want %v", c.from, c.to, got, c.ok)
		}
	}
}

func TestReserveReleaseCompleteDeadLetter(t *testing.T) {
	m := QueueMessage{ExecutionID: "e", Capability: "c", Priority: 5, Status: StatusQueued}
	if err := m.Reserve("w", 1, zeroTime()); err != nil {
		t.Fatal(err)
	}
	if m.Status != StatusReserved || m.LeaseGeneration != 1 {
		t.Fatalf("reserve not applied")
	}
	if err := m.Release(); err != nil {
		t.Fatal(err)
	}
	if m.Status != StatusQueued {
		t.Fatalf("release not applied")
	}
	if err := m.Reserve("w", 2, zeroTime()); err != nil {
		t.Fatal(err)
	}
	if err := m.Complete(); err != nil {
		t.Fatal(err)
	}
	m2 := QueueMessage{ExecutionID: "e2", Capability: "c", Priority: 5, Status: StatusQueued}
	m2.Reserve("w", 1, zeroTime())
	if err := m2.DeadLetter(); err != nil {
		t.Fatal(err)
	}
	if m2.Status != StatusDeadLetter {
		t.Fatalf("deadletter not applied")
	}
}

func TestInvariants(t *testing.T) {
	cases := []QueueMessage{
		{ExecutionID: "", Capability: "c", Priority: 0, Status: StatusQueued},
		{ExecutionID: "e", Capability: "", Priority: 0, Status: StatusQueued},
		{ExecutionID: "e", Capability: "c", Priority: 10, Status: StatusQueued},
		{ExecutionID: "e", Capability: "c", Priority: 0, Attempts: -1, Status: StatusQueued},
		{ExecutionID: "e", Capability: "c", Priority: 0, LeaseGeneration: -1, Status: StatusQueued},
	}
	for _, c := range cases {
		if err := c.ValidatesteInvariants(); err == nil {
			t.Errorf("expected error for %+v", c)
		}
	}
}

func zeroTime() time.Time { return time.Time{} }
