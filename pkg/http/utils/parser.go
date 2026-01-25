package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// RequestError wraps HTTP status and a user-friendly message
type RequestError struct {
	Status int
	Msg    string
}

func (e *RequestError) Error() string {
	return e.Msg
}

// UnknownFieldError wraps the field name for unknown field errors
type UnknownFieldError struct {
	Field string
}

func (e *UnknownFieldError) Error() string {
	return fmt.Sprintf("Request body contains unknown field %s", e.Field)
}

// DecodeJSON parses the JSON request body into the destination struct.
// It handles common edge cases like empty bodies, unknown fields, and malformed JSON.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) error {
	// 1. Check Content-Type
	ct := r.Header.Get("Content-Type")
	if ct != "" {
		mediaType := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
		if mediaType != "application/json" {
			return &RequestError{Status: http.StatusUnsupportedMediaType, Msg: "Content-Type header is not application/json"}
		}
	}

	// 2. Enforce Max Body Size (1MB)
	// This protects against denial-of-service attacks using large payloads.
	r.Body = http.MaxBytesReader(w, r.Body, 1048576)

	// 3. Configure Decoder
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields() // Strict schema validation

	// 4. Decode
	err := dec.Decode(dst)
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError

		switch {
		// Catch syntax errors
		case errors.As(err, &syntaxError):
			return &RequestError{Status: http.StatusBadRequest, Msg: fmt.Sprintf("Request body contains badly-formed JSON (at position %d)", syntaxError.Offset)}

		// Catch unexpected EOF (often due to malformed JSON)
		case errors.Is(err, io.ErrUnexpectedEOF):
			return &RequestError{Status: http.StatusBadRequest, Msg: "Request body contains badly-formed JSON"}

		// Catch type mismatches (e.g. string instead of int)
		case errors.As(err, &unmarshalTypeError):
			return unmarshalTypeError

		// Catch unknown fields (due to DisallowUnknownFields)
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return &UnknownFieldError{Field: fieldName}

		// Catch empty body
		case errors.Is(err, io.EOF):
			return &RequestError{Status: http.StatusBadRequest, Msg: "Request body must not be empty"}

		// Catch body too large
		case err.Error() == "http: request body too large":
			return &RequestError{Status: http.StatusRequestEntityTooLarge, Msg: "Request body must not be larger than 1MB"}

		default:
			return err
		}
	}

	// 5. Ensure only one JSON object
	err = dec.Decode(&struct{}{})
	if err != io.EOF {
		return &RequestError{Status: http.StatusBadRequest, Msg: "Request body must only contain a single JSON object"}
	}

	// 6. Validate the struct
	if err := ValidateStruct(dst); err != nil {
		return err
	}

	return nil
}
