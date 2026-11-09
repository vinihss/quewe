package app

import (
	"net/http"

	httpadapter "github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http"
	appconsuming "github.com/vinihss/quewe/services/quewe/internal/application/consuming"
	appworkers "github.com/vinihss/quewe/services/quewe/internal/application/workers"
)

func registerWorkersFeature(api *http.ServeMux, deps Dependencies) error {
	if deps.Queue == nil || deps.Executions == nil || deps.Workers == nil || deps.Auth == nil {
		return nil
	}
	cs := appconsuming.NewService(deps.Queue, deps.Executions, deps.Tasks, deps.Clock)
	ws := appworkers.NewService(deps.Workers, deps.Executions, deps.Clock)
	hw := httpadapter.NewWorkersHandler(ws, cs, deps.Auth)
	hw.Register(api)
	he := httpadapter.NewExecutionsBrokerHandler(cs, deps.Leasing, deps.Auth)
	he.Register(api)
	return nil
}
