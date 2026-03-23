package sudoku

import "errors"

// Sentinel errors returned by engine operations.
var (
	// ErrOutOfBounds is returned when a row or column index is outside [0, 8].
	ErrOutOfBounds = errors.New("sudoku: position out of bounds")

	// ErrInvalidValue is returned when a value is not in the range [1, 9].
	ErrInvalidValue = errors.New("sudoku: value must be between 1 and 9")

	// ErrCellLocked is returned when attempting to modify a cell that was set
	// during board initialisation (i.e. a puzzle "given").
	ErrCellLocked = errors.New("sudoku: cell is locked (puzzle given)")

	// ErrConflict is returned when a placement would violate Sudoku rules.
	// The number is not placed and the board is unchanged.
	ErrConflict = errors.New("sudoku: placement conflicts with existing values")
)
