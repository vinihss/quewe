package app

import (
	"net/http"

	httpadapter "github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http"
	apptasks "github.com/vinihss/quewe/services/quewe/internal/application/tasks"
)

// registerTaskFeature wires the WP2.2 task HTTP endpoints when dependencies are
// available. Registration is explicit (no feature framework magic).
func registerTaskFeature(api *http.ServeMux, deps Dependencies) error {
	if deps.Tasks == nil || deps.Auth == nil {
		return nil
	}
	svc := apptasks.NewService(deps.Tasks, deps.Clock, deps.IDs, deps.Cron)
	h := httpadapter.NewTasksHandler(svc, deps.Auth)
	h.Register(api)
	return nil
}
