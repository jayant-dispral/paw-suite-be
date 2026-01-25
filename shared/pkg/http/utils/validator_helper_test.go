package utils_test

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/jayant-dispral/brand-threat-be/shared/pkg/http/utils"
	"github.com/stretchr/testify/assert"
)

// TestStruct defines a struct with various validation tags for testing
type TestStruct struct {
	Name          string `json:"name" validate:"required"`
	Age           int    `json:"age" validate:"required,min=18"`
	Email         string `json:"email" validate:"required,email"`
	Role          string `json:"role" validate:"oneof=admin user"`
	ScanFrequency int    `json:"scan_frequency" validate:"required,min=5,max=60"`
}

func TestParseValidationError(t *testing.T) {
	validate := validator.New()

	t.Run("All fields missing", func(t *testing.T) {
		s := TestStruct{} // Empty struct
		err := validate.Struct(s)

		validationErrors := utils.ParseValidationError(err)

		assert.NotNil(t, validationErrors)
		assert.Len(t, validationErrors, 4) // Name, Age, Email, ScanFrequency are required. Role is not required but has oneof (empty string might pass oneof depending on config, but here it's just string)
		// Actually oneof on empty string usually fails if not pointer or omitempty.
		// Let's check specific fields.

		for _, ve := range validationErrors {
			assert.Equal(t, "REQUIRED", ve.Code)
			assert.Equal(t, "This field is required", ve.Message)
		}
	})

	t.Run("One field invalid (Min Value)", func(t *testing.T) {
		s := TestStruct{
			Name:          "Valid Name",
			Age:           10, // Invalid: min 18
			Email:         "test@example.com",
			Role:          "user",
			ScanFrequency: 10,
		}
		err := validate.Struct(s)

		validationErrors := utils.ParseValidationError(err)

		assert.Len(t, validationErrors, 1)
		assert.Equal(t, "age", validationErrors[0].Field)
		assert.Equal(t, "MIN_VALUE", validationErrors[0].Code)
		assert.Equal(t, "Must be at least 18", validationErrors[0].Message)
	})

	t.Run("Multiple validation errors", func(t *testing.T) {
		s := TestStruct{
			Name:          "Valid Name",
			Age:           20,
			Email:         "invalid-email", // Invalid Format
			Role:          "superadmin",    // Invalid Enum
			ScanFrequency: 100,             // Invalid Max
		}
		err := validate.Struct(s)

		validationErrors := utils.ParseValidationError(err)

		assert.Len(t, validationErrors, 3)

		// Check Email
		assert.Equal(t, "email", validationErrors[0].Field)
		assert.Equal(t, "INVALID_FORMAT", validationErrors[0].Code)

		// Check Role
		assert.Equal(t, "role", validationErrors[1].Field)
		assert.Equal(t, "INVALID_FORMAT", validationErrors[1].Code)

		// Check ScanFrequency
		assert.Equal(t, "scan_frequency", validationErrors[2].Field)
		assert.Equal(t, "MAX_VALUE", validationErrors[2].Code)
	})

	t.Run("Priority Check (Required vs Min)", func(t *testing.T) {
		// In Go validator, 0 is the zero value for int.
		// If we send 0 for ScanFrequency, it fails 'required' (because 0 is empty value) AND 'min=5'.
		// We want to ensure we get REQUIRED, not MIN_VALUE.
		s := TestStruct{
			Name:          "Valid Name",
			Age:           20,
			Email:         "test@example.com",
			Role:          "user",
			ScanFrequency: 0, // 0 implies missing/required fail for int
		}
		err := validate.Struct(s)

		validationErrors := utils.ParseValidationError(err)

		assert.Len(t, validationErrors, 1)
		assert.Equal(t, "scan_frequency", validationErrors[0].Field)
		assert.Equal(t, "REQUIRED", validationErrors[0].Code)
		assert.Equal(t, "This field is required", validationErrors[0].Message)
	})
}
