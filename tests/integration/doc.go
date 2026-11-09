// Package integration groups tests that need real infrastructure.
//
// Rode com: go test -tags=integration ./tests/...
//
// Guide rule (T4): integração usa Postgres real e `-race`; nada de banco
// fake para validatesr concorrência, lease ou transação.
package integration
