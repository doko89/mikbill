package usecase

// Error usecase dengan pesan Indonesia; dipetakan ke status HTTP di delivery.
type InputError struct{ Msg string }

func (e InputError) Error() string { return e.Msg }

type ConflictError struct{ Msg string }

func (e ConflictError) Error() string { return e.Msg }

type UnauthorizedError struct{ Msg string }

func (e UnauthorizedError) Error() string { return e.Msg }

type NotImplementedError struct{ Msg string }

func (e NotImplementedError) Error() string { return e.Msg }
