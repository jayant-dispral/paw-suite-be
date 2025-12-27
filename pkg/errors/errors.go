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
	return errors.Join(e.svcError, e.appErr).Error()
}

func (e Error) Is(target error) bool {
	return errors.Is(e.appErr, target) || errors.Is(e.svcError, target)
}
