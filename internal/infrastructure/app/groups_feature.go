package app

import (
	"net/http"

	httpadapter "github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http"
	appgroups "github.com/vinihss/quewe/services/quewe/internal/application/groups"
)

func registerGroupsFeature(api *http.ServeMux, deps Dependencies) error {
	if deps.Groups == nil || deps.Auth == nil {
		return nil
	}
	gs := appgroups.NewService(deps.Groups, deps.Clock, deps.IDs)
	hg := httpadapter.NewGroupsHandler(gs, deps.Auth)
	hg.Register(api)
	return nil
}
