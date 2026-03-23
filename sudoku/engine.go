// Package sudoku implements a Sudoku game engine.
//
// The engine exposes a Board type with the following public operations:
//
//   - PlaceNumber  – place a digit (1-9) at a position; refuses invalid moves.
//   - ClearCell    – remove a player-placed digit from a position.
//   - GetBoard     – return the current state as a 9×9 matrix.
//   - SetBoard     – overwrite the full board with a 9×9 matrix.
//
// Example:
//
//	b := sudoku.NewBoard()
//	ok, err := b.PlaceNumber(0, 0, 5)
//	matrix := b.GetBoard()
package sudoku

// BoardSize is the number of rows/columns in a standard Sudoku grid.
const BoardSize = 9

// BoxSize is the side-length of each 3×3 sub-box.
const BoxSize = 3

// cell holds a single square on the board.
type cell struct {
	value  int  // 0 = empty, 1-9 = filled
	locked bool // true when the value was set as part of puzzle initialisation
}

// Board represents the full 9×9 Sudoku grid and exposes the game engine API.
type Board struct {
	cells [BoardSize][BoardSize]cell
}

// NewBoard returns a new, empty Board ready for play.
func NewBoard() *Board {
	return &Board{}
}

// NewBoardFromMatrix creates a Board pre-populated from a 9×9 matrix.
// Non-zero values are treated as "givens" (locked cells).
// Returns an error if the matrix dimensions are wrong or any value is invalid /
// causes an immediate conflict.
func NewBoardFromMatrix(matrix [BoardSize][BoardSize]int) (*Board, error) {
	b := &Board{}
	return b, b.loadMatrix(matrix, true)
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// PlaceNumber attempts to place val (1-9) at (row, col).
//
// Returns:
//   - (true,  nil)  – placement succeeded.
//   - (false, err)  – placement was refused; the board is unchanged.
//     Possible errors: ErrOutOfBounds, ErrInvalidValue, ErrCellLocked, ErrConflict.
func (b *Board) PlaceNumber(row, col, val int) (bool, error) {
	if err := boundsCheck(row, col); err != nil {
		return false, err
	}
	if val < 1 || val > 9 {
		return false, ErrInvalidValue
	}
	if b.cells[row][col].locked {
		return false, ErrCellLocked
	}
	if !isValidPlacement(b.cells, row, col, val) {
		return false, ErrConflict
	}

	b.cells[row][col].value = val
	return true, nil
}

// PlaceNumberForce places val at (row, col) even if it conflicts with other
// cells. Used by the UI to allow the player to enter a wrong answer (which is
// then shown in red) rather than silently blocking the input.
// Returns ErrOutOfBounds, ErrInvalidValue, or ErrCellLocked on hard errors;
// returns nil (and writes the value) even when a conflict exists.
func (b *Board) PlaceNumberForce(row, col, val int) error {
	if err := boundsCheck(row, col); err != nil {
		return err
	}
	if val < 1 || val > 9 {
		return ErrInvalidValue
	}
	if b.cells[row][col].locked {
		return ErrCellLocked
	}
	b.cells[row][col].value = val
	return nil
}

// ClearCell removes a player-placed value from (row, col), setting it to 0.
//
// Returns an error if the coordinates are out of bounds or the cell is locked.
func (b *Board) ClearCell(row, col int) error {
	if err := boundsCheck(row, col); err != nil {
		return err
	}
	if b.cells[row][col].locked {
		return ErrCellLocked
	}
	b.cells[row][col].value = 0
	return nil
}

// IsSolved returns true when every cell is filled and no conflicts exist.
func (b *Board) IsSolved() bool {
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if b.cells[r][c].value == 0 {
				return false
			}
		}
	}
	// All cells filled — check there are no conflicts.
	conflicts := b.Conflicts()
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if conflicts[r][c] {
				return false
			}
		}
	}
	return true
}

// GetBoard returns the current board state as a 9×9 integer matrix.
// Empty cells are represented by 0; filled cells by their digit (1-9).
func (b *Board) GetBoard() [BoardSize][BoardSize]int {
	var matrix [BoardSize][BoardSize]int
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			matrix[r][c] = b.cells[r][c].value
		}
	}
	return matrix
}

// SetBoard replaces the entire board with the provided 9×9 matrix.
// All non-zero values are treated as new "givens" (locked cells).
// The operation is atomic: if any value is invalid or causes a conflict the
// board is left unchanged and an error is returned.
//
// Possible errors: ErrBoardSize, ErrInvalidValue, ErrConflict.
func (b *Board) SetBoard(matrix [BoardSize][BoardSize]int) error {
	return b.loadMatrix(matrix, true)
}

// LockedCells returns a 9×9 boolean matrix where true means the cell is a
// puzzle "given" (locked) and cannot be modified by the player.
func (b *Board) LockedCells() [BoardSize][BoardSize]bool {
	var m [BoardSize][BoardSize]bool
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			m[r][c] = b.cells[r][c].locked
		}
	}
	return m
}

// Conflicts returns a 9×9 boolean matrix where true means the cell's current
// value violates Sudoku rules (duplicate in its row, column, or 3×3 box).
// Empty cells (value 0) are never flagged.
func (b *Board) Conflicts() [BoardSize][BoardSize]bool {
	var m [BoardSize][BoardSize]bool
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			v := b.cells[r][c].value
			if v == 0 {
				continue
			}
			// Temporarily clear the cell so isValidPlacement checks neighbours.
			b.cells[r][c].value = 0
			if !isValidPlacement(b.cells, r, c, v) {
				m[r][c] = true
			}
			b.cells[r][c].value = v
		}
	}
	return m
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadMatrix validates then applies a 9×9 matrix to the board.
// When lock is true, every non-zero cell is marked as a puzzle given.
func (b *Board) loadMatrix(matrix [BoardSize][BoardSize]int, lock bool) error {
	// Validate all values before touching the board (atomic update).
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			v := matrix[r][c]
			if v < 0 || v > 9 {
				return ErrInvalidValue
			}
		}
	}

	// Build a fresh grid so we can validate conflicts before committing.
	var draft [BoardSize][BoardSize]cell
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			v := matrix[r][c]
			if v != 0 {
				if !isValidPlacement(draft, r, c, v) {
					return ErrConflict
				}
				draft[r][c] = cell{value: v, locked: lock}
			}
		}
	}

	b.cells = draft
	return nil
}

// boundsCheck returns ErrOutOfBounds if row or col is outside [0, BoardSize).
func boundsCheck(row, col int) error {
	if row < 0 || row >= BoardSize || col < 0 || col >= BoardSize {
		return ErrOutOfBounds
	}
	return nil
}

// applySolution writes a fully-solved matrix back into the board, overwriting
// only non-locked (player) cells and preserving locked-cell status.
// The matrix must be a complete, valid solution derived from this board.
func (b *Board) applySolution(solution [BoardSize][BoardSize]int) {
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if !b.cells[r][c].locked {
				b.cells[r][c].value = solution[r][c]
			}
		}
	}
}
