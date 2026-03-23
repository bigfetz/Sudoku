// Package ui implements the Fyne-based Sudoku GUI.
//
// BoardWidget is a fully custom fyne.Widget that renders the 9×9 grid.
// All sizing is dynamic — the widget fills whatever space the container gives
// it, so the board looks correct on both desktop and iPhone/iPad.
package ui

import (
	"image/color"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"sudoku_game/sudoku"
)

// border widths (logical pixels, independent of cell size)
const (
	thinBorder  float32 = 1
	thickBorder float32 = 3
	boardPad    float32 = thickBorder
	minCellSize float32 = 28 // smallest usable cell on a tiny screen
)

// totalBorderSpace returns the total pixels consumed by grid lines on one axis.
// 4 thick lines (outer 2 + 2 inner box dividers) + 6 thin lines (inner cell dividers)
func totalBorderSpace() float32 {
	return boardPad*2 + thickBorder*2 + thinBorder*6
}

// cellSizeFromBoardWidth derives the cell size that fits inside a given width.
func cellSizeFromBoardWidth(w float32) float32 {
	cs := (w - totalBorderSpace()) / float32(sudoku.BoardSize)
	if cs < minCellSize {
		cs = minCellSize
	}
	return cs
}

// boardSideFromCellSize is the inverse: total side length for a given cell size.
func boardSideFromCellSize(cs float32) float32 {
	return cs*float32(sudoku.BoardSize) + totalBorderSpace()
}

// ---- colours ---------------------------------------------------------------

// Light-mode palette (default).
var (
	colBackground   = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	colGridThin     = color.NRGBA{R: 0xAA, G: 0xAA, B: 0xAA, A: 0xFF}
	colGridThick    = color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF}
	colSelected     = color.NRGBA{R: 0xBB, G: 0xD7, B: 0xF8, A: 0xFF}
	colHighlight    = color.NRGBA{R: 0xDA, G: 0xEB, B: 0xFB, A: 0xFF}
	colLockedText   = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}
	colPlayerText   = color.NRGBA{R: 0x1A, G: 0x5F, B: 0xC8, A: 0xFF}
	colConflictBg   = color.NRGBA{R: 0xFC, G: 0xD0, B: 0xCE, A: 0xFF}
	colConflictText = color.NRGBA{R: 0xC0, G: 0x20, B: 0x20, A: 0xFF}
	colSameValue    = color.NRGBA{R: 0xA8, G: 0xC8, B: 0xF0, A: 0xFF} // other cells with the same digit

	// Flash colours.
	colFlashError = color.NRGBA{R: 0xFF, G: 0x45, B: 0x45, A: 0xFF} // red error fade
)

// SetDarkMode swaps all board colour variables to either the dark or light
// palette. Call bw.Refresh() after to redraw the board.
func SetDarkMode(dark bool) {
	if dark {
		colBackground = color.NRGBA{R: 0x1C, G: 0x1C, B: 0x1E, A: 0xFF} // near-black
		colGridThin = color.NRGBA{R: 0x48, G: 0x48, B: 0x50, A: 0xFF}
		colGridThick = color.NRGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}
		colSelected = color.NRGBA{R: 0x1A, G: 0x3A, B: 0x6A, A: 0xFF}
		colHighlight = color.NRGBA{R: 0x28, G: 0x2C, B: 0x38, A: 0xFF}
		colLockedText = color.NRGBA{R: 0xE8, G: 0xE8, B: 0xE8, A: 0xFF}
		colPlayerText = color.NRGBA{R: 0x5B, G: 0xB8, B: 0xFF, A: 0xFF}
		colConflictBg = color.NRGBA{R: 0x5A, G: 0x10, B: 0x10, A: 0xFF}
		colConflictText = color.NRGBA{R: 0xFF, G: 0x60, B: 0x60, A: 0xFF}
		colSameValue = color.NRGBA{R: 0x1E, G: 0x3A, B: 0x58, A: 0xFF}
	} else {
		colBackground = color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
		colGridThin = color.NRGBA{R: 0xAA, G: 0xAA, B: 0xAA, A: 0xFF}
		colGridThick = color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xFF}
		colSelected = color.NRGBA{R: 0xBB, G: 0xD7, B: 0xF8, A: 0xFF}
		colHighlight = color.NRGBA{R: 0xDA, G: 0xEB, B: 0xFB, A: 0xFF}
		colLockedText = color.NRGBA{R: 0x1A, G: 0x1A, B: 0x1A, A: 0xFF}
		colPlayerText = color.NRGBA{R: 0x1A, G: 0x5F, B: 0xC8, A: 0xFF}
		colConflictBg = color.NRGBA{R: 0xFC, G: 0xD0, B: 0xCE, A: 0xFF}
		colConflictText = color.NRGBA{R: 0xC0, G: 0x20, B: 0x20, A: 0xFF}
		colSameValue = color.NRGBA{R: 0xA8, G: 0xC8, B: 0xF0, A: 0xFF}
	}
}

// flashEntry tracks an in-progress cell animation.
// For error flashes it's a simple fade-out (wavePhase unused).
// For completion waves the colour is driven by a sine curve over wavePhase.
type flashEntry struct {
	r, g, b   uint8
	alpha     uint8   // used for error fade-out
	wave      bool    // true = wave animation, false = simple fade
	wavePhase float32 // 0..1 progress through the wave
	waveDelay int     // ticks to wait before starting (wave offset per cell)
}

// flashKey is the map key for a flashing cell.
type flashKey struct{ row, col int }

// ---- BoardWidget -----------------------------------------------------------

// BoardWidget is a focusable, interactive Sudoku board widget.
type BoardWidget struct {
	widget.BaseWidget

	board    *sudoku.Board
	selRow   int // -1 = nothing selected
	selCol   int
	conflict [sudoku.BoardSize][sudoku.BoardSize]bool

	// gridOX/gridOY are the pixel offsets from the widget origin to the top-left
	// corner of the drawn grid. Set by Layout; used by Tapped for hit-testing.
	gridOX, gridOY float32
	gridCS         float32 // cell size used in the most recent Layout pass

	// Flash animation state.
	flashMu    sync.Mutex
	flashCells map[flashKey]*flashEntry

	// solving is true while the auto-solver is placing numbers; suppresses
	// the blue selection/highlight colours so the board stays clean.
	solving bool

	// OnDigitCountsChanged is called (on the main goroutine) after every board
	// mutation so the numpad can hide exhausted digit buttons.
	OnDigitCountsChanged func()

	// OnSelectionChanged is called whenever the selected cell changes so the
	// numpad can show/hide the delete button.
	OnSelectionChanged func()

	// OnNewGame is called when a fresh puzzle is loaded (UpdateBoard).
	// Use it to reset external state like the timer.
	OnNewGame func()

	// OnSolved is called once when the puzzle is completed without conflicts.
	OnSolved func()

	// errorCount tracks how many wrong numbers the player has entered this game.
	errorCount int

	// OnErrorCountChanged is called whenever errorCount changes.
	OnErrorCountChanged func(int)
}

// NewBoardWidget creates a BoardWidget backed by the given engine Board.
func NewBoardWidget(b *sudoku.Board) *BoardWidget {
	bw := &BoardWidget{
		board:      b,
		selRow:     -1,
		selCol:     -1,
		flashCells: make(map[flashKey]*flashEntry),
	}
	bw.ExtendBaseWidget(bw)
	return bw
}

// UpdateBoard replaces the engine's board state with matrix and refreshes.
func (bw *BoardWidget) UpdateBoard(matrix [sudoku.BoardSize][sudoku.BoardSize]int) error {
	if err := bw.board.SetBoard(matrix); err != nil {
		return err
	}
	bw.conflict = bw.board.Conflicts()
	// Clear any lingering flashes when a whole new puzzle is loaded.
	bw.flashMu.Lock()
	bw.flashCells = make(map[flashKey]*flashEntry)
	bw.flashMu.Unlock()
	bw.selRow, bw.selCol = -1, -1
	bw.errorCount = 0
	if bw.OnErrorCountChanged != nil {
		bw.OnErrorCountChanged(0)
	}
	if bw.OnNewGame != nil {
		bw.OnNewGame()
	}
	bw.Refresh()
	bw.notifyDigitCounts()
	if bw.OnSelectionChanged != nil {
		bw.OnSelectionChanged()
	}
	return nil
}

// PlaceDigit places a digit in the currently selected cell (called by numpad).
func (bw *BoardWidget) PlaceDigit(digit int) {
	if bw.selRow < 0 {
		return
	}
	prevConflict := bw.board.Conflicts()
	// Use ForcePlace so a conflicting number is written to the board and shown
	// in red, rather than being silently rejected.
	err := bw.board.PlaceNumberForce(bw.selRow, bw.selCol, digit)
	bw.conflict = bw.board.Conflicts()

	if err == nil {
		row, col := bw.selRow, bw.selCol
		if bw.conflict[row][col] {
			// Conflict introduced — flash this cell red and count the error.
			bw.startFlash(row, col, colFlashError)
			bw.errorCount++
			if bw.OnErrorCountChanged != nil {
				bw.OnErrorCountChanged(bw.errorCount)
			}
		} else if !bw.solving {
			// No conflict — check whether a line/box just completed.
			// Skip during auto-solve; a whole-board wave fires at the end instead.
			bw.checkCompletions(prevConflict)
			// Check if the whole puzzle is now solved.
			if bw.board.IsSolved() && bw.OnSolved != nil {
				bw.OnSolved()
			}
		}
	}

	bw.Refresh()
	bw.notifyDigitCounts()
}

// ClearSelected clears the currently selected cell (called by numpad).
func (bw *BoardWidget) ClearSelected() {
	if bw.selRow < 0 {
		return
	}
	bw.board.ClearCell(bw.selRow, bw.selCol) //nolint:errcheck
	bw.conflict = bw.board.Conflicts()
	bw.Refresh()
	bw.notifyDigitCounts()
}

// DigitCounts returns how many times each digit 1-9 appears on the board.
// Index 0 is unused; index d holds the count for digit d.
func (bw *BoardWidget) DigitCounts() [10]int {
	var counts [10]int
	m := bw.board.GetBoard()
	for r := 0; r < sudoku.BoardSize; r++ {
		for c := 0; c < sudoku.BoardSize; c++ {
			if v := m[r][c]; v >= 1 && v <= 9 {
				counts[v]++
			}
		}
	}
	return counts
}

// notifyDigitCounts calls the registered callback if set.
func (bw *BoardWidget) notifyDigitCounts() {
	if bw.OnDigitCountsChanged != nil {
		bw.OnDigitCountsChanged()
	}
}

// ---- Flash / Wave animation ------------------------------------------------

const (
	flashTickInterval = 33 * time.Millisecond // ~30 fps

	flashDecay = uint8(18) // alpha per tick for error fade-out

	// Board-wide wave: total duration = waveDurationTicks * flashTickInterval.
	// waveFrontWidth controls how many "diagonal steps" the coloured band spans
	// at once — larger = more cells lit simultaneously.
	waveDurationTicks = 75  // ~2.5 s total (including tail fade)
	waveFrontWidth    = 6.0 // diagonal width of the colour band (in cell units)
)

// startFlash begins a simple fade-out error flash on (row,col).
func (bw *BoardWidget) startFlash(row, col int, base color.NRGBA) {
	key := flashKey{row, col}
	entry := &flashEntry{r: base.R, g: base.G, b: base.B, alpha: 0xFF, wave: false}
	bw.flashMu.Lock()
	bw.flashCells[key] = entry
	bw.flashMu.Unlock()

	go func() {
		for {
			time.Sleep(flashTickInterval)
			bw.flashMu.Lock()
			cur, ok := bw.flashCells[key]
			if !ok || cur != entry {
				bw.flashMu.Unlock()
				return
			}
			if cur.alpha <= flashDecay {
				delete(bw.flashCells, key)
				bw.flashMu.Unlock()
				fyne.Do(bw.Refresh)
				return
			}
			cur.alpha -= flashDecay
			bw.flashMu.Unlock()
			fyne.Do(bw.Refresh)
		}
	}()
}

// startWave starts a per-cell wave used for row/col/box completions.
// delayIdx staggers each cell so the wave ripples across the group.
func (bw *BoardWidget) startWave(row, col, delayIdx int) {
	// For group completions we use a lightweight per-cell approach:
	// each cell runs its own sine pulse, staggered by delayIdx ticks.
	const (
		cellWaveTicks   = 22 // ticks for one cell's sine pulse (~0.7 s)
		cellWaveSpacing = 2  // ticks between consecutive cells
	)
	key := flashKey{row, col}
	delay := delayIdx * cellWaveSpacing
	entry := &flashEntry{wave: true, waveDelay: delay}
	bw.flashMu.Lock()
	bw.flashCells[key] = entry
	bw.flashMu.Unlock()

	go func() {
		total := cellWaveTicks + delay
		for t := 0; t < total; t++ {
			time.Sleep(flashTickInterval)
			bw.flashMu.Lock()
			cur, ok := bw.flashCells[key]
			if !ok || cur != entry {
				bw.flashMu.Unlock()
				return
			}
			activeTick := t - delay + 1
			if activeTick > 0 {
				entry.wavePhase = float32(activeTick) / float32(cellWaveTicks)
				if entry.wavePhase > 1.0 {
					entry.wavePhase = 1.0
				}
			}
			bw.flashMu.Unlock()
			fyne.Do(bw.Refresh)
		}
		bw.flashMu.Lock()
		if cur, ok := bw.flashCells[key]; ok && cur == entry {
			delete(bw.flashCells, key)
		}
		bw.flashMu.Unlock()
		fyne.Do(bw.Refresh)
	}()
}

// WaveAllCells fires a single diagonal sweep wave from top-left to
// bottom-right across every cell. One shared goroutine drives all 81 cells
// via a continuous phase function — no per-cell goroutines needed.
func (bw *BoardWidget) WaveAllCells() {
	// maxDist is the diagonal length (top-left=0, bottom-right=16)
	const maxDist = float32((sudoku.BoardSize - 1) * 2) // 16

	go func() {
		for tick := 0; tick <= waveDurationTicks; tick++ {
			time.Sleep(flashTickInterval)

			// waveFront travels from -waveFrontWidth to maxDist+waveFrontWidth
			// over the full duration.
			totalTravel := maxDist + waveFrontWidth*2
			front := (float32(tick)/float32(waveDurationTicks))*totalTravel - waveFrontWidth

			bw.flashMu.Lock()
			for row := 0; row < sudoku.BoardSize; row++ {
				for col := 0; col < sudoku.BoardSize; col++ {
					dist := float32(row + col) // 0 (top-left) → 16 (bottom-right)
					// How far this cell is from the wave front (negative = ahead, positive = behind).
					offset := front - dist
					// Normalise into 0-1 within the front width band.
					phase := (offset + waveFrontWidth) / (waveFrontWidth * 2)
					if phase < 0 {
						phase = 0
					}
					if phase > 1 {
						phase = 1
					}

					key := flashKey{row, col}
					if tick == waveDurationTicks {
						// Final tick — remove all entries so board returns to normal.
						delete(bw.flashCells, key)
					} else {
						e, exists := bw.flashCells[key]
						if !exists || e.wave {
							if phase > 0 && phase < 1 {
								bw.flashCells[key] = &flashEntry{wave: true, wavePhase: phase}
							} else if exists && e.wave {
								delete(bw.flashCells, key)
							}
						}
					}
				}
			}
			bw.flashMu.Unlock()
			fyne.Do(bw.Refresh)
		}
	}()
}

// waveColour maps phase 0→1 through: white → light green → dark green → light green → white
func waveColour(phase float32) color.NRGBA {
	// Use a sine curve peaking at phase=0.5
	// sin(phase * π) gives 0 at both ends and 1 at the middle.
	s := float32(math.Sin(float64(phase) * math.Pi))

	// At peak (s=1): dark green  R=30  G=140 B=50
	// At s=0.5:      light green R=144 G=238 B=144
	// Blend: white (255,255,255) → light green → dark green → light green → white
	// We use two-stage: s<0.5 = white→lightGreen, s>0.5 = lightGreen→darkGreen (and back)
	darkG := [3]float32{30, 140, 50}
	lightG := [3]float32{144, 238, 144}
	white := [3]float32{255, 255, 255}

	var rgb [3]float32
	if s <= 0.5 {
		// white → light green (s: 0→0.5, t: 0→1)
		t := s * 2
		for i := 0; i < 3; i++ {
			rgb[i] = white[i] + t*(lightG[i]-white[i])
		}
	} else {
		// light green → dark green → light green (s: 0.5→1→0.5 mapped via sin again)
		t := (s - 0.5) * 2 // 0→1
		inner := float32(math.Sin(float64(t) * math.Pi))
		for i := 0; i < 3; i++ {
			rgb[i] = lightG[i] + inner*(darkG[i]-lightG[i])
		}
	}
	return color.NRGBA{R: uint8(rgb[0]), G: uint8(rgb[1]), B: uint8(rgb[2]), A: 0xFF}
}

// flashColour returns the overlay colour for (row,col), or nil if not flashing.
func (bw *BoardWidget) flashColour(row, col int) *color.NRGBA {
	bw.flashMu.Lock()
	e, ok := bw.flashCells[flashKey{row, col}]
	bw.flashMu.Unlock()
	if !ok {
		return nil
	}
	var c color.NRGBA
	if e.wave {
		if e.waveDelay > 0 {
			return nil // not started yet
		}
		c = waveColour(e.wavePhase)
	} else {
		c = color.NRGBA{R: e.r, G: e.g, B: e.b, A: e.alpha}
	}
	return &c
}

// checkCompletions scans rows, cols, and boxes. For any group that just became
// complete and conflict-free, it fires a wave animation across those cells.
func (bw *BoardWidget) checkCompletions(prevConflict [sudoku.BoardSize][sudoku.BoardSize]bool) {
	m := bw.board.GetBoard()
	conflicts := bw.conflict

	isComplete := func(cells [][2]int) bool {
		seen := make(map[int]bool, 9)
		for _, rc := range cells {
			v := m[rc[0]][rc[1]]
			if v == 0 || conflicts[rc[0]][rc[1]] {
				return false
			}
			seen[v] = true
		}
		return len(seen) == 9
	}

	wasComplete := func(cells [][2]int) bool {
		seen := make(map[int]bool, 9)
		for _, rc := range cells {
			v := m[rc[0]][rc[1]]
			if v == 0 || prevConflict[rc[0]][rc[1]] {
				return false
			}
			seen[v] = true
		}
		return len(seen) == 9
	}

	fireWave := func(cells [][2]int) {
		for i, rc := range cells {
			bw.startWave(rc[0], rc[1], i)
		}
	}

	// Check all rows.
	for r := 0; r < sudoku.BoardSize; r++ {
		cells := make([][2]int, sudoku.BoardSize)
		for c := 0; c < sudoku.BoardSize; c++ {
			cells[c] = [2]int{r, c}
		}
		if isComplete(cells) && !wasComplete(cells) {
			fireWave(cells)
		}
	}

	// Check all columns.
	for c := 0; c < sudoku.BoardSize; c++ {
		cells := make([][2]int, sudoku.BoardSize)
		for r := 0; r < sudoku.BoardSize; r++ {
			cells[r] = [2]int{r, c}
		}
		if isComplete(cells) && !wasComplete(cells) {
			fireWave(cells)
		}
	}

	// Check all 3×3 boxes.
	for br := 0; br < sudoku.BoxSize; br++ {
		for bc := 0; bc < sudoku.BoxSize; bc++ {
			cells := make([][2]int, 0, 9)
			for dr := 0; dr < sudoku.BoxSize; dr++ {
				for dc := 0; dc < sudoku.BoxSize; dc++ {
					cells = append(cells, [2]int{br*sudoku.BoxSize + dr, bc*sudoku.BoxSize + dc})
				}
			}
			if isComplete(cells) && !wasComplete(cells) {
				fireWave(cells)
			}
		}
	}
}

// ---- fyne.Widget -----------------------------------------------------------

func (bw *BoardWidget) CreateRenderer() fyne.WidgetRenderer {
	r := &boardRenderer{bw: bw}
	r.build()
	return r
}

// ---- desktop.Keyable -------------------------------------------------------

var _ desktop.Keyable = (*BoardWidget)(nil)

func (bw *BoardWidget) KeyDown(ev *fyne.KeyEvent) {
	if bw.selRow < 0 {
		return
	}
	switch ev.Name {
	case fyne.KeyBackspace, fyne.KeyDelete:
		bw.ClearSelected()
	case fyne.KeyUp:
		if bw.selRow > 0 {
			bw.selRow--
			bw.Refresh()
		}
	case fyne.KeyDown:
		if bw.selRow < sudoku.BoardSize-1 {
			bw.selRow++
			bw.Refresh()
		}
	case fyne.KeyLeft:
		if bw.selCol > 0 {
			bw.selCol--
			bw.Refresh()
		}
	case fyne.KeyRight:
		if bw.selCol < sudoku.BoardSize-1 {
			bw.selCol++
			bw.Refresh()
		}
	default:
		if d := keyToDigit(ev.Name); d != 0 {
			bw.PlaceDigit(d)
		}
	}
}
func (bw *BoardWidget) KeyUp(_ *fyne.KeyEvent) {}

// ---- fyne.Tappable ---------------------------------------------------------

var _ fyne.Tappable = (*BoardWidget)(nil)

func (bw *BoardWidget) Tapped(ev *fyne.PointEvent) {
	// Subtract the centering offset so the position is relative to the
	// top-left corner of the actual grid, then hit-test against cell geometry.
	adjusted := fyne.NewPos(ev.Position.X-bw.gridOX, ev.Position.Y-bw.gridOY)
	r, c := posToCellDynamic(adjusted, bw.gridCS)
	if r < 0 || c < 0 {
		return
	}
	bw.selRow, bw.selCol = r, c
	bw.Refresh()
	if bw.OnSelectionChanged != nil {
		bw.OnSelectionChanged()
	}
}

func (bw *BoardWidget) TappedSecondary(_ *fyne.PointEvent) {}

// ---- fyne.Focusable --------------------------------------------------------

var _ fyne.Focusable = (*BoardWidget)(nil)

func (bw *BoardWidget) FocusGained() { bw.Refresh() }
func (bw *BoardWidget) FocusLost()   { bw.Refresh() }
func (bw *BoardWidget) TypedRune(r rune) {
	if r >= '1' && r <= '9' && bw.selRow >= 0 {
		bw.PlaceDigit(int(r - '0'))
	}
}
func (bw *BoardWidget) TypedKey(ev *fyne.KeyEvent) { bw.KeyDown(ev) }

// ---- geometry helpers ------------------------------------------------------

// offsetForIndex returns the pixel distance from the grid origin to cell idx,
// using the given cell size.
func offsetForIndex(idx int, cs float32) float32 {
	acc := float32(0)
	for i := 0; i < idx; i++ {
		acc += cs
		if (i+1)%sudoku.BoxSize == 0 {
			acc += thickBorder
		} else {
			acc += thinBorder
		}
	}
	return acc
}

// cellOriginDynamic returns the top-left corner of cell (row,col) within the
// widget for the given cell size.
func cellOriginDynamic(row, col int, cs float32) (float32, float32) {
	return boardPad + offsetForIndex(col, cs), boardPad + offsetForIndex(row, cs)
}

// posToCellDynamic maps a tap position to a (row,col) using the given cell size.
func posToCellDynamic(pos fyne.Position, cs float32) (int, int) {
	col := pixelToIndexDynamic(pos.X-boardPad, cs)
	row := pixelToIndexDynamic(pos.Y-boardPad, cs)
	return row, col
}

func pixelToIndexDynamic(px, cs float32) int {
	acc := float32(0)
	for i := 0; i < sudoku.BoardSize; i++ {
		if i > 0 {
			if i%sudoku.BoxSize == 0 {
				acc += thickBorder
			} else {
				acc += thinBorder
			}
		}
		if px >= acc && px < acc+cs {
			return i
		}
		acc += cs
	}
	return -1
}

func keyToDigit(k fyne.KeyName) int {
	switch k {
	case fyne.Key1:
		return 1
	case fyne.Key2:
		return 2
	case fyne.Key3:
		return 3
	case fyne.Key4:
		return 4
	case fyne.Key5:
		return 5
	case fyne.Key6:
		return 6
	case fyne.Key7:
		return 7
	case fyne.Key8:
		return 8
	case fyne.Key9:
		return 9
	}
	return 0
}

// ---- boardRenderer ---------------------------------------------------------

type boardRenderer struct {
	bw *BoardWidget

	bg         *canvas.Rectangle
	cellBgs    [sudoku.BoardSize][sudoku.BoardSize]*canvas.Rectangle
	cellLabels [sudoku.BoardSize][sudoku.BoardSize]*canvas.Text
	lines      []fyne.CanvasObject
}

func (r *boardRenderer) build() {
	r.bg = canvas.NewRectangle(colBackground)
	for row := 0; row < sudoku.BoardSize; row++ {
		for col := 0; col < sudoku.BoardSize; col++ {
			bg := canvas.NewRectangle(colBackground)
			lbl := canvas.NewText("", colLockedText)
			lbl.Alignment = fyne.TextAlignCenter
			lbl.TextStyle = fyne.TextStyle{Bold: true}
			r.cellBgs[row][col] = bg
			r.cellLabels[row][col] = lbl
		}
	}
	// Pre-allocate line objects (positions set in Layout).
	// 2*(BoardSize+1) lines total.
	total := 2 * (sudoku.BoardSize + 1)
	r.lines = make([]fyne.CanvasObject, total)
	for i := range r.lines {
		r.lines[i] = canvas.NewRectangle(colGridThin)
	}
}

// Layout is called by Fyne every time the widget is resized.
// We derive cell size from the actual allocated size so the board is always
// square and fills the available width.
func (r *boardRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.FillColor = colBackground

	// Use the smaller dimension to keep the grid square.
	side := size.Width
	if size.Height < side {
		side = size.Height
	}
	cs := cellSizeFromBoardWidth(side)
	boardSide := boardSideFromCellSize(cs)

	// Centre the board if the container gives us more room than needed.
	ox := (size.Width - boardSide) / 2
	oy := (size.Height - boardSide) / 2

	// Persist offsets and cell size so Tapped can use the same geometry.
	r.bw.gridOX = ox
	r.bw.gridOY = oy
	r.bw.gridCS = cs

	matrix := r.bw.board.GetBoard()
	lockedMatrix := r.bw.board.LockedCells()
	fontSize := cs * 0.52
	lblHeight := fontSize * 1.4

	for row := 0; row < sudoku.BoardSize; row++ {
		for col := 0; col < sudoku.BoardSize; col++ {
			cx, cy := cellOriginDynamic(row, col, cs)
			cx += ox
			cy += oy

			bg := r.cellBgs[row][col]
			bg.Move(fyne.NewPos(cx, cy))
			bg.Resize(fyne.NewSize(cs, cs))
			bg.FillColor = r.cellBgColour(row, col)

			lbl := r.cellLabels[row][col]
			lbl.TextSize = fontSize
			lbl.Move(fyne.NewPos(cx, cy+(cs-lblHeight)/2))
			lbl.Resize(fyne.NewSize(cs, lblHeight))

			v := matrix[row][col]
			if v == 0 {
				lbl.Text = ""
			} else {
				lbl.Text = string(rune('0' + v))
			}
			if r.bw.conflict[row][col] {
				lbl.Color = colConflictText
			} else if lockedMatrix[row][col] {
				lbl.Color = colLockedText
			} else {
				lbl.Color = colPlayerText
			}
			lbl.Refresh()
		}
	}

	// Lay out grid lines.
	lineIdx := 0

	// Horizontal lines: i=0 is the top outer border, i=9 is the bottom outer border.
	for i := 0; i <= sudoku.BoardSize; i++ {
		thick := i == 0 || i == sudoku.BoardSize || i%sudoku.BoxSize == 0
		w := borderWidth(thick)
		var y float32
		if i == 0 {
			// Top outer border: flush with the top of the board.
			y = oy
		} else if i == sudoku.BoardSize {
			// Bottom outer border: flush with the bottom of the board.
			y = oy + boardSide - w
		} else {
			// Inner line: positioned after the cells above it.
			y = oy + boardPad + offsetForIndex(i, cs)
		}
		ln := r.lines[lineIdx].(*canvas.Rectangle)
		ln.FillColor = lineColour(thick)
		ln.Move(fyne.NewPos(ox, y))
		ln.Resize(fyne.NewSize(boardSide, w))
		lineIdx++
	}

	// Vertical lines: i=0 is the left outer border, i=9 is the right outer border.
	for i := 0; i <= sudoku.BoardSize; i++ {
		thick := i == 0 || i == sudoku.BoardSize || i%sudoku.BoxSize == 0
		w := borderWidth(thick)
		var x float32
		if i == 0 {
			// Left outer border: flush with the left of the board.
			x = ox
		} else if i == sudoku.BoardSize {
			// Right outer border: flush with the right of the board.
			x = ox + boardSide - w
		} else {
			// Inner line: positioned after the cells to its left.
			x = ox + boardPad + offsetForIndex(i, cs)
		}
		ln := r.lines[lineIdx].(*canvas.Rectangle)
		ln.FillColor = lineColour(thick)
		ln.Move(fyne.NewPos(x, oy))
		ln.Resize(fyne.NewSize(w, boardSide))
		lineIdx++
	}
}

func (r *boardRenderer) cellBgColour(row, col int) color.Color {
	bw := r.bw

	// Determine the base background colour first.
	var base color.NRGBA
	if bw.conflict[row][col] {
		base = colConflictBg
	} else if !bw.solving && bw.selRow == row && bw.selCol == col {
		base = colSelected
	} else if !bw.solving && bw.selRow >= 0 {
		sr, sc := bw.selRow, bw.selCol
		matrix := bw.board.GetBoard()
		selVal := matrix[sr][sc]

		if selVal != 0 && matrix[row][col] == selVal {
			base = colSameValue
		} else {
			sameBox := (row/sudoku.BoxSize == sr/sudoku.BoxSize) &&
				(col/sudoku.BoxSize == sc/sudoku.BoxSize)
			if row == sr || col == sc || sameBox {
				base = colHighlight
			} else {
				base = colBackground
			}
		}
	} else {
		base = colBackground
	}

	// Blend in any active flash overlay using its alpha.
	if flash := bw.flashColour(row, col); flash != nil {
		a := float32(flash.A) / 255.0
		blend := func(fb, bb uint8) uint8 {
			return uint8(float32(fb)*a + float32(bb)*(1-a))
		}
		return color.NRGBA{
			R: blend(flash.R, base.R),
			G: blend(flash.G, base.G),
			B: blend(flash.B, base.B),
			A: 0xFF,
		}
	}
	return base
}

// MinSize returns the smallest the board can reasonably be.
func (r *boardRenderer) MinSize() fyne.Size {
	side := boardSideFromCellSize(minCellSize)
	return fyne.NewSize(side, side)
}

func (r *boardRenderer) Refresh() {
	r.Layout(r.bw.Size())
	canvas.Refresh(r.bw)
}

func (r *boardRenderer) Destroy() {}

func (r *boardRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.bg}
	for row := 0; row < sudoku.BoardSize; row++ {
		for col := 0; col < sudoku.BoardSize; col++ {
			objs = append(objs, r.cellBgs[row][col])
		}
	}
	for row := 0; row < sudoku.BoardSize; row++ {
		for col := 0; col < sudoku.BoardSize; col++ {
			objs = append(objs, r.cellLabels[row][col])
		}
	}
	objs = append(objs, r.lines...)
	return objs
}

func lineColour(thick bool) color.Color {
	if thick {
		return colGridThick
	}
	return colGridThin
}

func borderWidth(thick bool) float32 {
	if thick {
		return thickBorder
	}
	return thinBorder
}
