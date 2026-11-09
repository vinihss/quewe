package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/vinihss/quewe/services/quewe/internal/adapters/inbound/http/dto"
	"github.com/vinihss/quewe/services/quewe/internal/application/apperr"
	"github.com/vinihss/quewe/services/quewe/internal/ports/repositories"
)

// errInvalidBody marks transport-level decoding failures.
var errInvalidBody = errors.New("invalid request body")

// Auth-related sentinels mapped by this adapter (ADR-001, WP2.2).
var (
	errAuthRequired      = errors.New("http: authentication required")
	errAuthNotConfigured = errors.New("http: authentication not configured")
)

const maxJSONBodyBytes = 256 << 10 // 256 KiB (ADR-009)

// writeJSON writes a JSON response with the correct content type.
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if body == nil {
		return
	}
	_ = json.NewEncoder(w).Encode(body)
}

// writeError translates domain/application/transport errors into the stable
// envelope {error:{code,message,details}} (spec 11, ADR-002).
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	status := http.StatusInternalServerError
	code := "internal"
	message := "internal error"
	var details map[string]any

	var ae *apperr.Error

	switch {
	case errors.Is(err, errInvalidBody):
		status, code, message = http.StatusBadRequest, "validatestion_error", "invalid request body"

	case errors.Is(err, errAuthRequired):
		status, code, message = http.StatusUnauthorized, "unauthorized", "authentication required"

	case errors.Is(err, errAuthNotConfigured):
		// Fail closed: real token verification not configured (WP2.2). Debug stub
		// must be explicitly enabled in APP_ENV=dev; never bypass in prod.
		status, code, message = http.StatusForbidden, "forbidden", "authentication is not configured; use APP_ENV=dev with DEBUG_AUTH_STUB=true for local development or configure a token resolver (ADR-001)"

	case errors.As(err, &ae):
		status, code, message, details = mapAppError(ae)

	case errors.Is(err, repositories.ErrNotFound):
		status, code, message = http.StatusNotFound, "not_found", "resource not found"

	case errors.Is(err, repositories.ErrConflict):
		status, code, message = http.StatusConflict, "conflict", "resource conflict"
	}

	if status >= http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed", "error", err, "path", r.URL.Path)
	}

	writeJSON(w, status, dto.ErrorResponse{Error: dto.ErrorBody{Code: code, Message: message, Details: details}})
}

func mapAppError(ae *apperr.Error) (status int, code, message string, details map[string]any) {
	switch ae.Code {
	case apperr.CodeValidatestion:
		// Application/domain validatestion failures are semantically 422; transport
		// decode failures are mapped separately as 400.
		return http.StatusUnprocessableEntity, "validatestion_error", ae.Error(), nil
	case apperr.CodeNotFound:
		return http.StatusNotFound, "not_found", ae.Error(), nil
	case apperr.CodeConflict:
		// ADR-014 requests including existing task id when available; WP2.1 app/port
		// do not return the conflicting id, so we return stable conflict + message.
		return http.StatusConflict, "conflict", ae.Error(), map[string]any{"hint": "conflicting resource id unavailable in WP2.2; see ADR-014/WP2.3"}
	case apperr.CodeNotSupported:
		// Spec 11 does not define not_supported; cron-without-calculator (ADR-003)
		// is a request/feature validatestion condition in WP2.2, mapped to stable 422.
		return http.StatusUnprocessableEntity, "validatestion_error", ae.Error(), nil
	case apperr.CodeInvalidTransition:
		return http.StatusConflict, "invalid_transition", ae.Error(), nil
	case apperr.CodeLeaseExpired:
		return http.StatusConflict, "lease_expired", ae.Error(), nil
	case apperr.CodeCapabilityNotAuth:
		return http.StatusForbidden, "capability_not_authorized", ae.Error(), nil
	default:
		return http.StatusInternalServerError, "internal", "internal error", nil
	}
}

// decodeJSON decodes a request body with a size limit and strict fields.
func decodeJSON(w http.ResponseWriter, r *http.Request, target any, maxBytes int64) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("%w: %v", errInvalidBody, err)
	}
	// Reject trailing data after a single JSON value.
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("%w: trailing data", errInvalidBody)
		}
		// A decode error after a successful first decode is unusual; treat as invalid.
		return fmt.Errorf("%w: %v", errInvalidBody, err)
	}
	return nil
}

func applyBodyLimit(w http.ResponseWriter, r *http.Request, maxBytes int64) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
}
