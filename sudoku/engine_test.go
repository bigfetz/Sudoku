package sudoku_test

import (
	"testing"

	"sudoku_game/sudoku"
)

// ---------------------------------------------------------------------------
// NewBoard / NewBoardFromMatrix
// ---------------------------------------------------------------------------

func TestNewBoard_EmptyBoard(t *testing.T) {
	b := sudoku.NewBoard()
	m := b.GetBoard()
	for r := 0; r < sudoku.BoardSize; r++ {
		for c := 0; c < sudoku.BoardSize; c++ {
			if m[r][c] != 0 {
				t.Errorf("expected empty board at [%d][%d], got %d", r, c, m[r][c])
			}
		}
	}
}

func TestNewBoardFromMatrix_Valid(t *testing.T) {
	var m [sudoku.BoardSize][sudoku.BoardSize]int
	m[0][0] = 5
	m[0][1] = 3

	b, err := sudoku.NewBoardFromMatrix(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := b.GetBoard()
	if got[0][0] != 5 || got[0][1] != 3 {
		t.Errorf("matrix not loaded correctly, got %v", got[0])
	}
}

func TestNewBoardFromMatrix_Conflict(t *testing.T) {
	var m [sudoku.BoardSize][sudoku.BoardSize]int
	m[0][0] = 5
	m[0][1] = 5 // duplicate in same row

	_, err := sudoku.NewBoardFromMatrix(m)
	if err != sudoku.ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// PlaceNumber
// ---------------------------------------------------------------------------

func TestPlaceNumber_ValidPlacement(t *testing.T) {
	b := sudoku.NewBoard()
	ok, err := b.PlaceNumber(0, 0, 5)
	if !ok || err != nil {
		t.Fatalf("expected valid placement, got ok=%v err=%v", ok, err)
	}
	if b.GetBoard()[0][0] != 5 {
		t.Fatal("value not written to board")
	}
}

func TestPlaceNumber_OutOfBounds(t *testing.T) {
	b := sudoku.NewBoard()
	ok, err := b.PlaceNumber(9, 0, 5)
	if ok || err != sudoku.ErrOutOfBounds {
		t.Fatalf("expected ErrOutOfBounds, got ok=%v err=%v", ok, err)
	}
}

func TestPlaceNumber_InvalidValue(t *testing.T) {
	b := sudoku.NewBoard()
	ok, err := b.PlaceNumber(0, 0, 10)
	if ok || err != sudoku.ErrInvalidValue {
		t.Fatalf("expected ErrInvalidValue, got ok=%v err=%v", ok, err)
	}
}

func TestPlaceNumber_RowConflict(t *testing.T) {
	b := sudoku.NewBoard()
	b.PlaceNumber(0, 0, 5)
	ok, err := b.PlaceNumber(0, 1, 5)
	if ok || err != sudoku.ErrConflict {
		t.Fatalf("expected ErrConflict for row duplicate, got ok=%v err=%v", ok, err)
	}
}

func TestPlaceNumber_ColConflict(t *testing.T) {
	b := sudoku.NewBoard()
	b.PlaceNumber(0, 0, 5)
	ok, err := b.PlaceNumber(1, 0, 5)
	if ok || err != sudoku.ErrConflict {
		t.Fatalf("expected ErrConflict for col duplicate, got ok=%v err=%v", ok, err)
	}
}

func TestPlaceNumber_BoxConflict(t *testing.T) {
	b := sudoku.NewBoard()
	b.PlaceNumber(0, 0, 5)
	ok, err := b.PlaceNumber(1, 1, 5) // same 3×3 box
	if ok || err != sudoku.ErrConflict {
		t.Fatalf("expected ErrConflict for box duplicate, got ok=%v err=%v", ok, err)
	}
}

func TestPlaceNumber_LockedCell(t *testing.T) {
	var m [sudoku.BoardSize][sudoku.BoardSize]int
	m[0][0] = 7
	b, _ := sudoku.NewBoardFromMatrix(m)

	ok, err := b.PlaceNumber(0, 0, 3)
	if ok || err != sudoku.ErrCellLocked {
		t.Fatalf("expected ErrCellLocked, got ok=%v err=%v", ok, err)
	}
}

// ---------------------------------------------------------------------------
// ClearCell
// ---------------------------------------------------------------------------

func TestClearCell_Success(t *testing.T) {
	b := sudoku.NewBoard()
	b.PlaceNumber(4, 4, 9)
	if err := b.ClearCell(4, 4); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.GetBoard()[4][4] != 0 {
		t.Fatal("cell was not cleared")
	}
}

func TestClearCell_LockedCell(t *testing.T) {
	var m [sudoku.BoardSize][sudoku.BoardSize]int
	m[2][3] = 4
	b, _ := sudoku.NewBoardFromMatrix(m)

	if err := b.ClearCell(2, 3); err != sudoku.ErrCellLocked {
		t.Fatalf("expected ErrCellLocked, got %v", err)
	}
}

func TestClearCell_OutOfBounds(t *testing.T) {
	b := sudoku.NewBoard()
	if err := b.ClearCell(-1, 0); err != sudoku.ErrOutOfBounds {
		t.Fatalf("expected ErrOutOfBounds, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetBoard / SetBoard
// ---------------------------------------------------------------------------

func TestGetBoard_ReturnsSnapshot(t *testing.T) {
	b := sudoku.NewBoard()
	b.PlaceNumber(3, 3, 6)
	m := b.GetBoard()
	// Mutating the returned matrix must not affect the board.
	m[3][3] = 0
	if b.GetBoard()[3][3] != 6 {
		t.Fatal("GetBoard returned a reference instead of a copy")
	}
}

func TestSetBoard_ValidMatrix(t *testing.T) {
	b := sudoku.NewBoard()
	b.PlaceNumber(0, 0, 1)

	var m [sudoku.BoardSize][sudoku.BoardSize]int
	m[8][8] = 9

	if err := b.SetBoard(m); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := b.GetBoard()
	if got[0][0] != 0 {
		t.Error("old value not cleared by SetBoard")
	}
	if got[8][8] != 9 {
		t.Error("new value not applied by SetBoard")
	}
}

func TestSetBoard_ConflictingMatrix(t *testing.T) {
	b := sudoku.NewBoard()
	var m [sudoku.BoardSize][sudoku.BoardSize]int
	m[0][0] = 3
	m[0][5] = 3 // same row conflict

	if err := b.SetBoard(m); err != sudoku.ErrConflict {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
	// Board must remain unchanged.
	if b.GetBoard()[0][0] != 0 {
		t.Error("board was mutated despite conflict error")
	}
}

func TestSetBoard_InvalidValue(t *testing.T) {
	b := sudoku.NewBoard()
	var m [sudoku.BoardSize][sudoku.BoardSize]int
	m[0][0] = 11

	if err := b.SetBoard(m); err != sudoku.ErrInvalidValue {
		t.Fatalf("expected ErrInvalidValue, got %v", err)
	}
}
