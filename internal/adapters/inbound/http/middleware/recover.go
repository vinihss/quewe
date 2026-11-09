package middleware

import (
	"log/slog"
	"net/http"
)

// Recover converts a panic into 500 without leaking the stack to the client.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}

			slog.ErrorContext(r.Context(), "panic recovered",
				"panic", recovered,
				"path", r.URL.Path,
				"request_id", RequestIDFrom(r.Context()),
			)

			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"code":"internal","message":"internal error"}`))
		}()

		next.ServeHTTP(w, r)
	})
}
