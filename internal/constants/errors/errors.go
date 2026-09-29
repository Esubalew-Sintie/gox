package errors

import (
	"errors"
)

var (
	ErrPathExists   = errors.New("path already exists")
	ErrPathNotFound = errors.New("path not found")

	ErrConfigNotFound = errors.New("gox config not found")
	ErrInvalidConfig  = errors.New("invalid gox config")
)
