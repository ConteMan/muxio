package api

import (
	"encoding/json"
	"net/http"
)

// Error codes. They are part of the published contract, so clients may branch
// on them; the message is for people and may change.
const (
	CodeInvalidArgument  = "invalid_argument"
	CodeNotFound         = "not_found"
	CodeMethodNotAllowed = "method_not_allowed"
	CodeInternal         = "internal"
)

// errorBody is the single error shape every endpoint returns.
type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

func writeError(w http.ResponseWriter, status int, code, message, field string) {
	writeJSON(w, status, errorBody{Error: code, Message: message, Field: field})
}

func invalidArgument(w http.ResponseWriter, field, message string) {
	writeError(w, http.StatusBadRequest, CodeInvalidArgument, message, field)
}

func notFound(w http.ResponseWriter, message string) {
	writeError(w, http.StatusNotFound, CodeNotFound, message, "")
}

// internalError hides the underlying cause from the response. Details belong in
// the log, not in a payload a browser can read.
func internalError(w http.ResponseWriter, logError func(error), err error) {
	if logError != nil {
		logError(err)
	}
	writeError(w, http.StatusInternalServerError, CodeInternal,
		"the request could not be completed", "")
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
