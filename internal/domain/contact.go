// Package domain holds entities and errors shared by all layers.
// It must not import any other package of this module.
package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound   = errors.New("contact not found")
	ErrPhoneTaken = errors.New("contact with this phone number already exists")
)

// ValidationError carries per-field validation messages.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string { return "validation failed" }

type Contact struct {
	ID        int64     `json:"id"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     string    `json:"phone"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ListFilter struct {
	Query  string // matches name or phone, case-insensitive substring
	Limit  int
	Offset int
}
