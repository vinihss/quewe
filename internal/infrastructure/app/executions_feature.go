package app

import (
	"net/http"

	httpadapter "github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http"
)

func registerExecutionsFeature(api *http.ServeMux, deps Dependencies) error {
	if deps.Executions == nil || deps.Auth == nil {
		return nil
	}
	h := httpadapter.NewExecutionsHandler(deps.Executions, deps.Auth)
	h.Register(api)
	return nil
}
