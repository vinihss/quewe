// Package middleware holds HTTP-level concerns shared by every route.
//
// Guide rule (I6): autenticação, correlação, logging e recuperação de panic
// são middleware. Nenhum handler repete esse código.
package middleware

import "net/http"

// Middleware wraps an http.Handler.
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares so the first argument is the outermost layer.
func Chain(h http.Handler, mw ...Middleware) http.Handler {
	for i := len(mw) - 1; i >= 0; i-- {
		h = mw[i](h)
	}
	return h
}
