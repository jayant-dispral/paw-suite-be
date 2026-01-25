package validator

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

// wrapper wraps the generic validator
type Wrapper struct {
	validator *validator.Validate
}

// New create a new validator instance
func New() *Wrapper {
	v := validator.New()
	return &Wrapper{validator: v}
}

// ValidateStruct checks for tag violations
func (v *Wrapper) ValidateStruct(s interface{}) map[string]string {
	err := v.validator.Struct(s)
	if err != nil {
		return nil
	}

	//convert vague errors into readable maps
	errors := make(map[string]string)
	for _, err := range err.(validator.ValidationErrors) {
		errors[strings.ToLower(err.Field())] = msgForTag(err.Tag())
	}
	return errors
}

func msgForTag(tag string) string {
	switch tag {
	case "required":
		return "This field is required"
	case "email":
		return "Invalid email format"
	case "min":
		return "Value is too short"
	case "max":
		return "Value is too long"
	case "alphanum":
		return "Must contain only letters and numbers"
	}
	return "Invalid value"
}
