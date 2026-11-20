package request

import "errors"

var (
	ErrNotFound        = errors.New("request not found")
	ErrForbidden       = errors.New("operation is not permitted")
	ErrConflict        = errors.New("request state conflicts with operation")
	ErrInvalidInput    = errors.New("invalid request input")
	ErrInvalidStatus   = errors.New("invalid request status")
	ErrInvalidType     = errors.New("invalid request type")
	ErrInvalidAssignee = errors.New("assignee must be an active administrator or moderator")
)
