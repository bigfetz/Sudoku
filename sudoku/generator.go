package sudoku

import (
	"math/rand"
)

// Difficulty controls how many givens remain in the generated puzzle.
type Difficulty int

const (
	VeryEasy Difficulty = 46 // ~46 givens — lots of numbers pre-filled
	Easy     Difficulty = 36 // ~36 givens
	Medium   Difficulty = 28 // ~28 givens
	Hard     Difficulty = 22 // ~22 givens
	VeryHard Difficulty = 17 // ~17 givens — bare minimum for a unique solution
)

// GenerateBoard returns a new, randomly generated Sudoku puzzle at the
// requested difficulty level. The returned matrix is guaranteed to have a
// unique solution. Non-zero values are givens; zeros are blanks for the player.
func GenerateBoard(d Difficulty) [BoardSize][BoardSize]int {
	// 1. Build a random complete solution.
	var solution [BoardSize][BoardSize]int
	fillBoard(&solution, rand.New(rand.NewSource(rand.Int63())))

	// 2. Remove cells while the puzzle keeps a unique solution.
	puzzle := solution
	removeCells(&puzzle, int(d))

	return puzzle
}

// ---------------------------------------------------------------------------
// Step 1 — fill a board with a random valid solution (backtracking)
// ---------------------------------------------------------------------------

// fillBoard fills cells recursively using a randomly-ordered candidate list.
// Returns true when the board is fully solved.
func fillBoard(b *[BoardSize][BoardSize]int, rng *rand.Rand) bool {
	row, col, found := nextEmpty(b)
	if !found {
		return true // all cells filled — solution complete
	}

	digits := shuffled(rng)
	for _, d := range digits {
		if isValidInMatrix(b, row, col, d) {
			b[row][col] = d
			if fillBoard(b, rng) {
				return true
			}
			b[row][col] = 0
		}
	}
	return false // backtrack
}

// ---------------------------------------------------------------------------
// Step 2 — remove cells while preserving a unique solution
// ---------------------------------------------------------------------------

// removeCells removes givens from a complete solution until only `givens`
// remain, ensuring each removal still leaves a unique solution.
func removeCells(b *[BoardSize][BoardSize]int, givens int) {
	// Build a shuffled list of all 81 positions.
	type pos struct{ r, c int }
	positions := make([]pos, 0, BoardSize*BoardSize)
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			positions = append(positions, pos{r, c})
		}
	}
	rand.Shuffle(len(positions), func(i, j int) {
		positions[i], positions[j] = positions[j], positions[i]
	})

	remaining := BoardSize * BoardSize // 81

	for _, p := range positions {
		if remaining <= givens {
			break
		}
		backup := b[p.r][p.c]
		b[p.r][p.c] = 0

		if countSolutions(b, 2) == 1 {
			// Safe to remove — puzzle still has exactly one solution.
			remaining--
		} else {
			// Restoring this cell is necessary for uniqueness.
			b[p.r][p.c] = backup
		}
	}
}

// SolveMatrix attempts to solve the 9×9 matrix b in place.
// count is set to 1 if a solution exists, 0 if not.
// This operates purely on a raw matrix — no Board required.
func SolveMatrix(b *[BoardSize][BoardSize]int, count *int) {
	*count = 0
	solveInPlace(b, count)
}

// solveInPlace is like solve but leaves the board filled with the first
// solution found instead of backtracking past it.
func solveInPlace(b *[BoardSize][BoardSize]int, count *int) bool {
	if *count >= 1 {
		return true
	}
	row, col, found := nextEmpty(b)
	if !found {
		*count++
		return true // board is fully filled with a valid solution
	}
	for d := 1; d <= BoardSize; d++ {
		if isValidInMatrix(b, row, col, d) {
			b[row][col] = d
			if solveInPlace(b, count) {
				return true // keep this value — don't backtrack
			}
			b[row][col] = 0
		}
	}
	return false
}

// Solve attempts to solve the puzzle currently loaded in b.
// It fills every empty cell with the unique solution value.
// Returns true if a solution was found, false if the puzzle is unsolvable.
// Locked cells (givens) are never modified.
func Solve(b *Board) bool {
	m := b.GetBoard()
	count := 0
	solve(&m, &count, 1)
	if count == 0 {
		return false
	}
	// Write solution back without changing locked-cell status.
	b.applySolution(m)
	return true
}

// countSolutions counts the number of solutions for b, stopping as soon as
// limit solutions have been found (avoids exhaustive search).
func countSolutions(b *[BoardSize][BoardSize]int, limit int) int {
	// Work on a copy so the original is unchanged.
	work := *b
	count := 0
	solve(&work, &count, limit)
	return count
}

func solve(b *[BoardSize][BoardSize]int, count *int, limit int) {
	if *count >= limit {
		return
	}
	row, col, found := nextEmpty(b)
	if !found {
		*count++
		return
	}
	for d := 1; d <= BoardSize; d++ {
		if isValidInMatrix(b, row, col, d) {
			b[row][col] = d
			solve(b, count, limit)
			b[row][col] = 0
		}
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// nextEmpty returns the first empty cell (value == 0) in reading order.
func nextEmpty(b *[BoardSize][BoardSize]int) (row, col int, found bool) {
	for r := 0; r < BoardSize; r++ {
		for c := 0; c < BoardSize; c++ {
			if b[r][c] == 0 {
				return r, c, true
			}
		}
	}
	return 0, 0, false
}

// isValidInMatrix reports whether placing val at (row,col) in matrix b is
// legal (no duplicate in the same row, column, or 3×3 box).
func isValidInMatrix(b *[BoardSize][BoardSize]int, row, col, val int) bool {
	for i := 0; i < BoardSize; i++ {
		if b[row][i] == val || b[i][col] == val {
			return false
		}
	}
	startRow := (row / BoxSize) * BoxSize
	startCol := (col / BoxSize) * BoxSize
	for r := startRow; r < startRow+BoxSize; r++ {
		for c := startCol; c < startCol+BoxSize; c++ {
			if b[r][c] == val {
				return false
			}
		}
	}
	return true
}

// shuffled returns [1..9] in a random order.
func shuffled(rng *rand.Rand) []int {
	digits := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	rng.Shuffle(len(digits), func(i, j int) {
		digits[i], digits[j] = digits[j], digits[i]
	})
	return digits
}
