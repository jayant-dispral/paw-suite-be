package utils

import (
	"errors"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/jayant-dispral/brand-threat-be/internal/core/domain"
	pkgErrors "github.com/jayant-dispral/brand-threat-be/pkg/errors"
)

var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register a function to get the JSON tag name instead of the struct field name
	// This ensures error messages use "email" instead of "Email"
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// ValidateStruct checks for validation errors and returns a domain-wrapped error
func ValidateStruct(s interface{}) error {
	err := validate.Struct(s)
	if err != nil {
		var validationErrors validator.ValidationErrors
		if errors.As(err, &validationErrors) {
			// Return the original validation error so it can be type-asserted upstream
			return validationErrors
		}
		return pkgErrors.NewError(domain.ErrInvalidInput, err)
	}
	return nil
}
