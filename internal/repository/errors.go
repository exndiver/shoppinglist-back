package repository

import "errors"

var (
	ErrNotFound      = errors.New("not found")
	ErrConflict      = errors.New("conflict")
	ErrDepthExceeded = errors.New("merge chain depth exceeded")
)
