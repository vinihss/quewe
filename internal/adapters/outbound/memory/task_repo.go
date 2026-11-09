// Package memory provides temporary in-memory adapters for WP2.x without DB.
// WP2.3 replaces this with Postgres adapters; do not treat as production storage.
package memory

import (
	"context"
	"encoding/base64"
	"sort"
	"sync"

	"github.com/vinihss/quewe/services/quewe/internal/domain/task"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

type TaskRepository struct {
	mu    sync.Mutex
	items []task.Definition
}

func NewTaskRepository() *TaskRepository {
	return &TaskRepository{}
}

func (r *TaskRepository) Create(_ context.Context, def task.Definition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, it := range r.items {
		if it.OwnerID == def.OwnerID && it.Name == def.Name {
			return repositories.ErrConflict
		}
	}
	r.items = append(r.items, def)
	return nil
}

func (r *TaskRepository) GetByID(_ context.Context, ownerID, id string) (task.Definition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, it := range r.items {
		if it.ID == id && it.OwnerID == ownerID {
			return it, nil
		}
	}
	return task.Definition{}, repositories.ErrNotFound
}

func (r *TaskRepository) GetByTaskID(_ context.Context, taskID string) (task.Definition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, it := range r.items {
		if it.ID == taskID {
			return it, nil
		}
	}
	return task.Definition{}, repositories.ErrNotFound
}

func (r *TaskRepository) List(_ context.Context, f repositories.TaskListFilter) (repositories.TaskPage, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var filtered []task.Definition
	for _, it := range r.items {
		if it.OwnerID != f.OwnerID {
			continue
		}
		if f.Enabled != nil && it.Enabled != *f.Enabled {
			continue
		}
		if f.Type != "" && it.Type != f.Type {
			continue
		}
		filtered = append(filtered, it)
	}
	sort.Slice(filtered, func(i, j int) bool {
		if filtered[i].CreatedAt.Equal(filtered[j].CreatedAt) {
			return filtered[i].ID < filtered[j].ID
		}
		return filtered[i].CreatedAt.After(filtered[j].CreatedAt)
	})
	start := 0
	if f.Cursor != "" {
		b, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		if err == nil {
			lastID := string(b)
			for i, it := range filtered {
				if it.ID == lastID {
					start = i + 1
					break
				}
			}
		}
	}
	if start > len(filtered) {
		start = len(filtered)
	}
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	end := start + limit
	if end > len(filtered) {
		end = len(filtered)
	}
	page := repositories.TaskPage{Items: append([]task.Definition(nil), filtered[start:end]...)}
	if end < len(filtered) && len(page.Items) > 0 {
		page.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(page.Items[len(page.Items)-1].ID))
	}
	return page, nil
}
