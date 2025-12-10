package domain

import "fmt"

// NotFoundError represents a missing resource.
type NotFoundError struct {
	Resource string
}

func (e NotFoundError) Error() string {
	if e.Resource == "" {
		return "not found"
	}
	return fmt.Sprintf("%s not found", e.Resource)
}

// Is enables errors.Is matching on NotFoundError.
func (e NotFoundError) Is(target error) bool {
	_, ok := target.(NotFoundError)
	if ok {
		return true
	}
	_, ok = target.(*NotFoundError)
	return ok
}

// ErrNotFound is the sentinel error for missing resources.
var ErrNotFound = NotFoundError{}
