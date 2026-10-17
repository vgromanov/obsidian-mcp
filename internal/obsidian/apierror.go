package obsidian

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrCodeUnsupportedContentType is the Local REST API error code for a request
// whose Content-Type the route does not accept ("Unknown or invalid
// Content-Type"). Local REST 5.x returns it for Dataview DQL on POST /search/.
const ErrCodeUnsupportedContentType = 40012

// APIError is a non-2xx response from the Local REST API. Error() keeps the
// "METHOD /path: body" text callers have always seen.
type APIError struct {
	Method string
	Path   string
	Status int
	// Code is the Local REST errorCode from the JSON body (0 when absent).
	Code int
	Body string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s %s: %s", e.Method, e.Path, e.Body)
}

func newAPIError(method, path string, status int, body []byte) *APIError {
	e := &APIError{Method: method, Path: path, Status: status, Body: summarizeErrorBody(body)}
	var payload struct {
		ErrorCode int `json:"errorCode"`
	}
	if err := json.Unmarshal(body, &payload); err == nil {
		e.Code = payload.ErrorCode
	}
	return e
}

// IsUnsupportedContentType reports whether err is a Local REST 40012 response.
func IsUnsupportedContentType(err error) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Code == ErrCodeUnsupportedContentType
}
