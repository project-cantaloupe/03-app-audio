package audio

import "errors"

var (
	ErrInvalidInput   = errors.New("invalid input")
	ErrUnauthorized   = errors.New("authentication required")
	ErrForbidden      = errors.New("access denied")
	ErrNotFound       = errors.New("audio not found")
	ErrNotReady       = errors.New("audio is not ready for playback")
	ErrObjectMismatch = errors.New("uploaded object does not match the request")
)
