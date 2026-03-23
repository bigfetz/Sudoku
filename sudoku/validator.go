package sudoku

// isValidPlacement reports whether placing val at (row, col) on the given
// cells grid is legal according to Sudoku rules (no duplicate in the row,
// column, or 3×3 box).
func isValidPlacement(cells [BoardSize][BoardSize]cell, row, col, val int) bool {
	return !conflictsInRow(cells, row, col, val) &&
		!conflictsInCol(cells, row, col, val) &&
		!conflictsInBox(cells, row, col, val)
}

// conflictsInRow checks whether val already appears in the given row,
// ignoring the cell at (row, col) itself.
func conflictsInRow(cells [BoardSize][BoardSize]cell, row, col, val int) bool {
	for c := 0; c < BoardSize; c++ {
		if c == col {
			continue
		}
		if cells[row][c].value == val {
			return true
		}
	}
	return false
}

// conflictsInCol checks whether val already appears in the given column,
// ignoring the cell at (row, col) itself.
func conflictsInCol(cells [BoardSize][BoardSize]cell, row, col, val int) bool {
	for r := 0; r < BoardSize; r++ {
		if r == row {
			continue
		}
		if cells[r][col].value == val {
			return true
		}
	}
	return false
}

// conflictsInBox checks whether val already appears in the 3×3 box that
// contains (row, col), ignoring the cell at (row, col) itself.
func conflictsInBox(cells [BoardSize][BoardSize]cell, row, col, val int) bool {
	startRow := (row / BoxSize) * BoxSize
	startCol := (col / BoxSize) * BoxSize

	for r := startRow; r < startRow+BoxSize; r++ {
		for c := startCol; c < startCol+BoxSize; c++ {
			if r == row && c == col {
				continue
			}
			if cells[r][c].value == val {
				return true
			}
		}
	}
	return false
}
