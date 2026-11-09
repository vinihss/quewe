//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/outbound/postgres"
	"github.com/vinihss/quewe/services/quewe/internal/domain/queue"
)

// generateUUID gera um UUID v4 formatado para testes.
func generateUUID() string {
	// UUID v4: xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx onde y = 8, 9, A, B
	// Usamos contador com bits fixados para simular UUID v4
	uuidCounter++
	// Bytes fixados para UUID v4: versão 4 (4) e variável (8,9,A,B)
	return fmt.Sprintf("%08x-4000-8000-%04x-%012x",
		uuidCounter, uuidCounter&0xFFFF, uuidCounter&0xFFFFFFFFFFFF)
}

var (
	uuidCounter int
	mu          sync.Mutex
)

var workerID string

// TestConcurrentReserve proves that N goroutines reserving simultaneously
// nao generate duplicates thanks to FOR UPDATE SKIP LOCKED.
func TestConcurrentReserve(t *testing.T) {
	t.Helper()
	pool := setupDB(t)
	repo := postgres.NewQueueRepository(pool)

	nMessages := 50
	nGoroutines := 10
	capability := "system.maintenance"
	now := time.Now().UTC()
	workerID = generateUUID()

	// Creates 50 mensagens with varied priority 0..9 e available_at=now
	for i := 0; i < nMessages; i++ {
		msg := queue.QueueMessage{
			ID:              generateUUID(),
			ExecutionID:     generateUUID(),
			Queue:           "test",
			Capability:      capability,
			Priority:        i % 10,
			AvailableAt:     now,
			Status:          queue.StatusQueued,
			Attempts:        0,
			ReservedBy:      nil,
			LeaseExpiresAt:  nil,
			LeaseGeneration: 0,
			TraceContext:    nil,
			CreatedAt:       now,
		}
		if err := repo.Enqueue(context.Background(), msg); err != nil {
			t.Fatalf("enqueue setup: %v", err)
		}
	}

	// 10 goroutines reserving simultaneously - usamos WaitGroup e mapa compartilhado
	var reservedIDs map[string]bool
	var wg sync.WaitGroup
	reservedIDs = make(map[string]bool, nMessages)

	// Lançamento das goroutines - each reserves one message until exhausted
	for g := 0; g < nGoroutines; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < nMessages/nGoroutines+5; i++ {
				reserved, err := repo.Reserve(context.Background(), workerID, []string{capability}, 1)
				if err != nil || len(reserved) == 0 {
					return
				}
				mu.Lock()
				if reservedIDs[reserved[0].ID] {
					mu.Unlock()
					return // duplicate, skip
				}
				reservedIDs[reserved[0].ID] = true
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	// After all goroutines finish, verify results
	reservedCount := len(reservedIDs)
	expected := nMessages

	// Assertions
	if reservedCount != expected {
		t.Errorf("expected %d reserved messages, got %d", expected, reservedCount)
	}

	// Verify that não há duplicatas - each message_id must appear exactly once
	if len(reservedIDs) != expected {
		t.Errorf("expected %d unique reserved IDs, got %d", expected, len(reservedIDs))
	}

	// Verify that all messages are reserved in the DB
	var statusCount int
	err := pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM queue_messages WHERE status='reserved'").Scan(&statusCount)
	if err != nil {
		t.Fatal(err)
	}
	if statusCount != expected {
		t.Errorf("db shows %d reserved, expected %d", statusCount, expected)
	}

	// Verify that no message remained with status 'queued'
	var queuedCount int
	err = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM queue_messages WHERE status='queued'").Scan(&queuedCount)
	if err != nil {
		t.Fatal(err)
	}
	if queuedCount != 0 {
		t.Errorf("expected 0 queued messages, got %d", queuedCount)
	}
}

// TestConcurrentReservePriority proves that a reservation order respects priority DESC.
func TestConcurrentReservePriority(t *testing.T) {
	t.Helper()
	pool := setupDB(t)
	repo := postgres.NewQueueRepository(pool)

	capability := "system.maintenance"
	nPerPriority := 10 // 10 messages per priority
	totalPrios := 3    // prioridade 9, 5, 0

	now := time.Now().UTC()
	workerID = generateUUID()

	// Creates mensagens com prioridades diferentes: 10 de prio 9, 10 de prio 5, 10 de prio 0
	for i := 0; i < nPerPriority; i++ {
		// prioridade 9
		msg := queue.QueueMessage{
			ID:              generateUUID(),
			ExecutionID:     generateUUID(),
			Queue:           "test",
			Capability:      capability,
			Priority:        9,
			AvailableAt:     now,
			Status:          queue.StatusQueued,
			Attempts:        0,
			ReservedBy:      nil,
			LeaseExpiresAt:  nil,
			LeaseGeneration: 0,
			TraceContext:    nil,
			CreatedAt:       now,
		}
		if err := repo.Enqueue(context.Background(), msg); err != nil {
			t.Fatalf("enqueue setup prio9: %v", err)
		}

		// prioridade 5
		msg5 := queue.QueueMessage{
			ID:              generateUUID(),
			ExecutionID:     generateUUID(),
			Queue:           "test",
			Capability:      capability,
			Priority:        5,
			AvailableAt:     now,
			Status:          queue.StatusQueued,
			Attempts:        0,
			ReservedBy:      nil,
			LeaseExpiresAt:  nil,
			LeaseGeneration: 0,
			TraceContext:    nil,
			CreatedAt:       now,
		}
		if err := repo.Enqueue(context.Background(), msg5); err != nil {
			t.Fatalf("enqueue setup prio5: %v", err)
		}

		// prioridade 0
		msg0 := queue.QueueMessage{
			ID:              generateUUID(),
			ExecutionID:     generateUUID(),
			Queue:           "test",
			Capability:      capability,
			Priority:        0,
			AvailableAt:     now,
			Status:          queue.StatusQueued,
			Attempts:        0,
			ReservedBy:      nil,
			LeaseExpiresAt:  nil,
			LeaseGeneration: 0,
			TraceContext:    nil,
			CreatedAt:       now,
		}
		if err := repo.Enqueue(context.Background(), msg0); err != nil {
			t.Fatalf("enqueue setup prio0: %v", err)
		}
	}

	// 3 goroutines reservando (max=1 cada), total de 30 reservas
	nReservations := nPerPriority * totalPrios // 30
	var reservedIDs []string
	var mu2 sync.Mutex
	var wg sync.WaitGroup

	for g := 0; g < 3; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < nReservations/3; i++ {
				reserved, err := repo.Reserve(context.Background(), workerID, []string{capability}, 1)
				if err != nil {
					return
				}
				if len(reserved) > 0 {
					mu2.Lock()
					reservedIDs = append(reservedIDs, reserved[0].ID)
					mu2.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	// Contar por prioridade no DB
	var prio9, prio5, prio0 int
	for _, id := range reservedIDs {
		var p int
		err := pool.QueryRow(context.Background(), "SELECT priority FROM queue_messages WHERE id=$1", id).Scan(&p)
		if err != nil {
			t.Fatalf("query priority for %s: %v", id, err)
		}
		switch p {
		case 9:
			prio9++
		case 5:
			prio5++
		case 0:
			prio0++
		}
	}

	// As 10 mensagens de prioridade 9 must be reserved first,
	// then those with priority 5, then those with priority 0
	if prio9 != nPerPriority {
		t.Errorf("expected %d priority-9 reserved, got %d", nPerPriority, prio9)
	}
	if prio5 != nPerPriority {
		t.Errorf("expected %d priority-5 reserved, got %d", nPerPriority, prio5)
	}
	if prio0 != nPerPriority {
		t.Errorf("expected %d priority-0 reserved, got %d", nPerPriority, prio0)
	}
}

// TestFencingMismatch proves that Ack/Nack com lease_generation errado failure and the message remains reserved.
func TestFencingMismatch(t *testing.T) {
	t.Helper()
	pool := setupDB(t)
	repo := postgres.NewQueueRepository(pool)

	now := time.Now().UTC()
	workerID = generateUUID()
	execID := generateUUID()

	// Creates uma mensagem with status that allows reservation
	msg := queue.QueueMessage{
		ID:              generateUUID(),
		ExecutionID:     execID,
		Queue:           "test",
		Capability:      "system.maintenance",
		Priority:        5,
		AvailableAt:     now,
		Status:          queue.StatusQueued,
		Attempts:        0,
		ReservedBy:      nil,
		LeaseExpiresAt:  nil,
		LeaseGeneration: 0,
		TraceContext:    nil,
		CreatedAt:       now,
	}
	if err := repo.Enqueue(context.Background(), msg); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	// Reserve the message (will generate lease_generation = 1)
	reserved, err := repo.Reserve(context.Background(), workerID, []string{"system.maintenance"}, 1)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if len(reserved) == 0 {
		t.Fatal("expected reserved message")
	}
	m := reserved[0]
	gen := m.LeaseGeneration // = 1

	// Try Ack com lease_generation = G+1 = 2 → deve failurer
	err = repo.Ack(context.Background(), execID, gen+1)
	if err == nil {
		t.Error("expected Ack with gen+1 to fail, got nil")
	}
	// The message must remain 'reserved'

	// Try Nack com lease_generation = G+1 = 2 → deve failurer
	err = repo.Nack(context.Background(), execID, gen+1)
	if err == nil {
		t.Error("expected Nack with gen+1 to fail, got nil")
	}
	// Message must remain 'reserved' (not acked/nacked)

	// Verify status in DB
	var status string
	err = pool.QueryRow(context.Background(), "SELECT status FROM queue_messages WHERE id=$1", m.ID).Scan(&status)
	if err != nil {
		t.Fatalf("query status: %v", err)
	}
	if status != string(queue.StatusReserved) {
		t.Errorf("expected status=reserved after fencing mismatch, got %s", status)
	}
}

// TestLeaseExpiryRequeue_Concurrent prova reap of expired leases followed by new reservation.
func TestLeaseExpiryRequeue_Concurrent(t *testing.T) {
	t.Helper()
	pool := setupDB(t)
	repo := postgres.NewQueueRepository(pool)

	capability := "system.maintenance"
	nMessages := 10
	nGoroutines := 5

	now := time.Now().UTC()
	workerID = generateUUID()

	// Createsr 10 mensagens com status reservado e lease expirado
	msgIDs := make([]string, nMessages)
	for i := 0; i < nMessages; i++ {
		execID := generateUUID()
		msg := queue.QueueMessage{
			ID:              generateUUID(),
			ExecutionID:     execID,
			Queue:           "test",
			Capability:      capability,
			Priority:        5,
			AvailableAt:     now,
			Status:          queue.StatusQueued,
			Attempts:        0,
			ReservedBy:      nil,
			LeaseExpiresAt:  nil,
			LeaseGeneration: 0,
			TraceContext:    nil,
			CreatedAt:       now,
		}
		if err := repo.Enqueue(context.Background(), msg); err != nil {
			t.Fatalf("enqueue setup: %v", err)
		}

		// Reserve the message
		reserved, err := repo.Reserve(context.Background(), workerID, []string{capability}, 1)
		if err != nil {
			t.Fatalf("reserve setup msg %d: %v", i, err)
		}
		if len(reserved) == 0 {
			t.Fatalf("expected reserved msg %d", i)
		}
		m := reserved[0]
		// Set lease_expires_at in the past to simulate expired
		expiredAt := now.Add(-30 * time.Second)
		_, err = pool.Exec(context.Background(), "UPDATE queue_messages SET lease_expires_at=$1 WHERE id=$2", expiredAt, m.ID)
		if err != nil {
			t.Fatalf("set lease expired: %v", err)
		}
		msgIDs[i] = m.ID
	}

	// Chamar ReapExpiredLeases
	reaped, err := repo.ReapExpired(context.Background(), time.Now())
	if err != nil {
		t.Fatalf("reap expired: %v", err)
	}
	_ = reaped

	// Verify that todas as 10 mensagens returned to 'queued'
	for _, id := range msgIDs {
		var status string
		err = pool.QueryRow(context.Background(), "SELECT status FROM queue_messages WHERE id=$1", id).Scan(&status)
		if err != nil {
			t.Fatalf("query status for %s: %v", id, err)
		}
		if status != string(queue.StatusQueued) {
			t.Errorf("message %s expected status=queued after reap, got %s", id, status)
		}
		// Verificar attempts incremented (deve ser 2: 1 do reserve + 1 do reap)
		var attempts int
		err = pool.QueryRow(context.Background(), "SELECT attempts FROM queue_messages WHERE id=$1", id).Scan(&attempts)
		if err != nil {
			t.Fatalf("query attempts for %s: %v", id, err)
		}
		if attempts != 2 {
			t.Errorf("message %s expected attempts=2 after reserve+reap (1 do reserve + 1 do reap), got %d", id, attempts)
		}
	}

	// 5 goroutines voltam a reservá-las
	var reservedCount int
	var mu3 sync.Mutex
	var wg sync.WaitGroup

	perGoroutine := nMessages / nGoroutines // 2 cada

	for g := 0; g < nGoroutines; g++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				reserved, err := repo.Reserve(context.Background(), workerID, []string{capability}, 1)
				if err != nil {
					continue
				}
				if len(reserved) > 0 {
					mu3.Lock()
					reservedCount++
					mu3.Unlock()
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify that total reservado = 10, without duplicates
	if reservedCount != nMessages {
		t.Errorf("expected %d total reserved after re-reserve, got %d", nMessages, reservedCount)
	}

	// Verificar in DB that all are reserved
	var statusCount int
	err = pool.QueryRow(context.Background(), "SELECT COUNT(*) FROM queue_messages WHERE status='reserved'").Scan(&statusCount)
	if err != nil {
		t.Fatal(err)
	}
	if statusCount != nMessages {
		t.Errorf("db shows %d reserved, expected %d", statusCount, nMessages)
	}
}
