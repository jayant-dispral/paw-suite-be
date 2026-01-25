package errors

import "errors"

type Error struct {
	appErr   error
	svcError error
}

func NewError(appErr, svcErr error) Error {
	return Error{
		appErr:   appErr,
		svcError: svcErr,
	}
}

func (e Error) Error() string {
	// Return the service error message (detail) if available for better API responses.
	if e.svcError != nil {
		return e.svcError.Error()
	}
	return e.appErr.Error()
}

// Unwrap returns the underlying service error to allow errors.As to work
func (e Error) Unwrap() error {
	return e.svcError
}

func (e Error) Is(target error) bool {
	return errors.Is(e.appErr, target) || errors.Is(e.svcError, target)
}
