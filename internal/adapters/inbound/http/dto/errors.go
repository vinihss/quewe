package dto

// ErrorBody is the stable machine-readable error payload (spec 11).
type ErrorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// ErrorResponse is the stable error envelope.
type ErrorResponse struct {
	Error ErrorBody `json:"error"`
}
