package domain

import "errors"

// Sentinel errors returned by repositories and use cases. Adapters wrap
// them with fmt.Errorf("...: %w") and callers test with errors.Is.
var (
	ErrNotFound      = errors.New("not found")
	ErrAlreadyExists = errors.New("already exists")
	ErrInvalid       = errors.New("invalid input")
)
