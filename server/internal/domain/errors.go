package domain

import "errors"

// Error domain.
var (
	ErrNotFound      = errors.New("tidak ditemukan")
	ErrAlreadyExists = errors.New("sudah ada")
)
