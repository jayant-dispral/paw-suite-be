package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"unicode"

	"github.com/go-playground/validator/v10"
)

// ValidationErrorResponse defines the structure for a single validation error
type ValidationErrorResponse struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ParseValidationError checks if the error is a validator error and formats it
func ParseValidationError(err error) []ValidationErrorResponse {
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		// 1. Group errors by field to handle deduplication and priority
		fieldErrors := make(map[string][]validator.FieldError)
		var fieldOrder []string // Keep track of order for consistent response

		for _, fe := range ve {
			path := getFieldPath(fe)
			if _, exists := fieldErrors[path]; !exists {
				fieldOrder = append(fieldOrder, path)
			}
			fieldErrors[path] = append(fieldErrors[path], fe)
		}

		out := make([]ValidationErrorResponse, 0, len(fieldOrder))

		// 2. Process each field and select the most important error
		for _, path := range fieldOrder {
			errs := fieldErrors[path]
			selectedErr := selectPriorityError(errs)
			if shouldSkipValidationError(selectedErr) {
				continue
			}

			out = append(out, ValidationErrorResponse{
				Field:   path,
				Code:    getErrorCode(selectedErr),
				Message: msgForTag(selectedErr),
			})
		}
		return out
	}

	// Handle JSON Type Mismatch Errors (e.g. string instead of int)
	var ute *json.UnmarshalTypeError
	if errors.As(err, &ute) {
		return []ValidationErrorResponse{
			{
				Field:   ute.Field,
				Code:    "INVALID_TYPE",
				Message: fmt.Sprintf("Expected %s value but got %s", ute.Type, ute.Value),
			},
		}
	}

	// Handle Unknown Field Errors
	var ufe *UnknownFieldError
	if errors.As(err, &ufe) {
		return []ValidationErrorResponse{
			{
				Field:   ufe.Field,
				Code:    "UNKNOWN_FIELD",
				Message: "This field is not allowed",
			},
		}
	}

	return nil
}

// selectPriorityError enforces the order: Required -> Data Types -> Value Constraints
func selectPriorityError(errs []validator.FieldError) validator.FieldError {
	// Priority 1: Required
	for _, fe := range errs {
		if fe.Tag() == "required" {
			return fe
		}
	}

	// Priority 2: Data Types / Format (e.g. Email, URL, OneOf)
	for _, fe := range errs {
		switch fe.Tag() {
		case "email", "url", "fqdn", "oneof", "timezone":
			return fe
		}
	}

	// Priority 3: Value Constraints (Min, Max, etc.) - Default to the first one found
	return errs[0]
}

func getFieldPath(fe validator.FieldError) string {
	ns := fe.StructNamespace()
	if idx := strings.Index(ns, "."); idx != -1 {
		ns = ns[idx+1:]
	}
	return toSnakePath(ns)
}

func shouldSkipValidationError(fe validator.FieldError) bool {
	if !isEmptyValue(fe.Value()) {
		return false
	}

	switch fe.Tag() {
	case "email", "url", "fqdn", "oneof", "timezone":
		return true
	default:
		return false
	}
}

func isEmptyValue(value interface{}) bool {
	if value == nil {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.String, reflect.Array, reflect.Slice, reflect.Map:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return rv.IsNil()
	}
	return false
}

func toSnakePath(value string) string {
	parts := strings.Split(value, ".")
	for i, part := range parts {
		parts[i] = toSnake(part)
	}
	return strings.Join(parts, ".")
}

func toSnake(value string) string {
	var b strings.Builder
	for i, r := range value {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func getErrorCode(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "REQUIRED"
	case "email", "url", "fqdn", "timezone":
		return "INVALID_FORMAT"
	case "oneof":
		return "INVALID_FORMAT" // Enum mismatch is treated as format issue
	case "min", "gte", "gt":
		return "MIN_VALUE"
	case "max", "lte", "lt":
		return "MAX_VALUE"
	default:
		return "INVALID_FORMAT"
	}
}

func msgForTag(fe validator.FieldError) string {
	switch fe.Tag() {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "url":
		return "Invalid URL format"
	case "fqdn":
		return "Invalid domain name format"
	case "oneof":
		return fmt.Sprintf("Value must be one of: %s", strings.ReplaceAll(fe.Param(), " ", ", "))
	case "min":
		return fmt.Sprintf("Must be at least %s", fe.Param())
	case "max":
		return fmt.Sprintf("Must be at most %s", fe.Param())
	default:
		return fmt.Sprintf("Failed validation on tag '%s'", fe.Tag())
	}
}
