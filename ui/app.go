package ui

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"image/color"

	"sudoku_game/sudoku"
)

var (
	colErrorLabel       = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF} // grey when zero
	colErrorLabelActive = color.NRGBA{R: 0xC0, G: 0x20, B: 0x20, A: 0xFF} // red when errors > 0
)

// setStatColours updates the stats-row colour variables for the current mode.
func setStatColours(dark bool) {
	if dark {
		colErrorLabel = color.NRGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xFF}
		colErrorLabelActive = color.NRGBA{R: 0xFF, G: 0x60, B: 0x60, A: 0xFF}
	} else {
		colErrorLabel = color.NRGBA{R: 0x88, G: 0x88, B: 0x88, A: 0xFF}
		colErrorLabelActive = color.NRGBA{R: 0xC0, G: 0x20, B: 0x20, A: 0xFF}
	}
}

// gameTimer drives a running mm:ss clock displayed in the stats row.
// It supports pause/resume so the clock freezes when the app backgrounds.
type gameTimer struct {
	label   *canvas.Text
	elapsed time.Duration // accumulated time before the current run
	startAt time.Time     // when the current run started (zero if paused/stopped)
	stopCh  chan struct{}
	paused  bool
	done    bool // true after stop() — never resumes
	mu      sync.Mutex
}

func (gt *gameTimer) start() {
	gt.mu.Lock()
	gt.elapsed = 0
	gt.done = false
	gt.paused = false
	gt.startAt = time.Now()
	gt.stopCh = make(chan struct{})
	gt.mu.Unlock()
	gt.runTicker()
}

func (gt *gameTimer) runTicker() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				gt.mu.Lock()
				total := gt.elapsed + time.Since(gt.startAt)
				gt.mu.Unlock()
				m := int(total.Minutes())
				s := int(total.Seconds()) % 60
				text := fmt.Sprintf("%d:%02d", m, s)
				fyne.Do(func() {
					gt.label.Text = text
					gt.label.Refresh()
				})
			case <-gt.stopCh:
				return
			}
		}
	}()
}

func (gt *gameTimer) pause() {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	if gt.paused || gt.done {
		return
	}
	gt.paused = true
	gt.elapsed += time.Since(gt.startAt)
	close(gt.stopCh)
}

func (gt *gameTimer) resume() {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	if !gt.paused || gt.done {
		return
	}
	gt.paused = false
	gt.startAt = time.Now()
	gt.stopCh = make(chan struct{})
	go gt.runTicker()
}

func (gt *gameTimer) reset() {
	gt.mu.Lock()
	if !gt.paused && !gt.done {
		close(gt.stopCh)
	}
	gt.done = false
	gt.paused = false
	gt.elapsed = 0
	gt.startAt = time.Now()
	gt.stopCh = make(chan struct{})
	gt.mu.Unlock()
	fyne.Do(func() {
		gt.label.Text = "0:00"
		gt.label.Refresh()
	})
	gt.runTicker()
}

func (gt *gameTimer) stop() {
	gt.mu.Lock()
	defer gt.mu.Unlock()
	if gt.done {
		return
	}
	gt.done = true
	if !gt.paused {
		close(gt.stopCh)
	}
}

// Run creates the Fyne application, builds the main window, and blocks until
// the window is closed.
func Run() {
	a := app.New()
	a.Settings().SetTheme(theme.LightTheme())

	w := a.NewWindow("Sudoku")

	board := sudoku.NewBoard()
	_ = board.SetBoard(starterPuzzle())

	bw := NewBoardWidget(board)
	bw.conflict = board.Conflicts()

	// ---- Toolbar -----------------------------------------------------------

	newBtn := widget.NewButton("New Game", func() {
		showNewGameDialog(w, bw)
	})
	newBtn.Importance = widget.HighImportance

	solveBtn := widget.NewButton("Solve", func() {
		showSolveConfirm(w, bw)
	})
	solveBtn.Importance = widget.HighImportance

	// Dark-mode toggle — starts in light mode.
	// onThemeChange is assigned below once the stats labels exist.
	darkMode := false
	var onThemeChange func()
	themeBtn := widget.NewButton("🌙", func() {
		if onThemeChange != nil {
			onThemeChange()
		}
	})

	toolbar := container.NewHBox(
		widget.NewLabel("Sudoku"),
		layout.NewSpacer(),
		themeBtn,
		solveBtn,
		newBtn,
	)

	// ---- On-screen number pad (essential for touch/iOS) --------------------
	numpad, refreshNumpad := buildNumpad(bw)
	bw.OnDigitCountsChanged = refreshNumpad

	// ---- Error counter -----------------------------------------------------
	errorLabel := canvas.NewText("Mistakes: 0", colErrorLabel)
	errorLabel.TextSize = 16
	errorLabel.TextStyle = fyne.TextStyle{Bold: true}
	errorLabel.Alignment = fyne.TextAlignCenter
	bw.OnErrorCountChanged = func(n int) {
		if n == 0 {
			errorLabel.Text = "Mistakes: 0"
			errorLabel.Color = colErrorLabel
		} else {
			errorLabel.Text = fmt.Sprintf("Mistakes: %d", n)
			errorLabel.Color = colErrorLabelActive
		}
		errorLabel.Refresh()
	}

	// ---- Timer -------------------------------------------------------------
	timerLabel := canvas.NewText("0:00", colErrorLabel)
	timerLabel.TextSize = 16
	timerLabel.TextStyle = fyne.TextStyle{Bold: true}
	timerLabel.Alignment = fyne.TextAlignCenter

	// Now that both stat labels exist, wire up the theme toggle callback.
	onThemeChange = func() {
		darkMode = !darkMode
		if darkMode {
			a.Settings().SetTheme(theme.DarkTheme())
			themeBtn.SetText("☀️")
		} else {
			a.Settings().SetTheme(theme.LightTheme())
			themeBtn.SetText("🌙")
		}
		SetDarkMode(darkMode)
		setStatColours(darkMode)
		errorLabel.Color = colErrorLabel
		errorLabel.Refresh()
		timerLabel.Color = colErrorLabel
		timerLabel.Refresh()
		bw.Refresh()
	}

	gt := &gameTimer{label: timerLabel}
	gt.start()

	// Pause/resume the timer when the app backgrounds/foregrounds.
	a.Lifecycle().SetOnExitedForeground(func() { gt.pause() })
	a.Lifecycle().SetOnEnteredForeground(func() { gt.resume() })

	// Reset timer and mistakes when a new game starts.
	bw.OnNewGame = func() {
		gt.reset()
	}

	// Stop timer when puzzle is solved.
	bw.OnSolved = func() {
		gt.stop()
	}

	// ---- Full layout -------------------------------------------------------
	// Border layout: toolbar top, numpad bottom, board fills the rest.
	statsRow := container.NewHBox(
		layout.NewSpacer(),
		errorLabel,
		widget.NewLabel("  |  "),
		timerLabel,
		layout.NewSpacer(),
	)
	content := container.NewBorder(
		container.NewVBox(toolbar, widget.NewSeparator(), statsRow),
		container.NewVBox(widget.NewSeparator(), numpad),
		nil, nil,
		bw, // board fills centre — Fyne stretches it to all available space
	)

	w.SetContent(content)
	w.Canvas().Focus(bw)

	// Stop the timer cleanly when the window is closed.
	w.SetCloseIntercept(func() {
		gt.stop()
		w.Close()
	})

	w.ShowAndRun()
}

// tallSlot wraps a button in a fixed-height container so the numpad row is
// taller than the default button height, and hidden buttons still occupy space.
func tallSlot(btn fyne.CanvasObject) fyne.CanvasObject {
	const btnHeight = float32(72)
	return container.NewStack(
		// Invisible spacer that enforces the minimum height.
		newMinSizeBox(fyne.NewSize(0, btnHeight)),
		widget.NewLabel(""), // keeps slot visible when button hidden
		btn,
	)
}

// newMinSizeBox returns a canvas object whose MinSize is the given size.
func newMinSizeBox(size fyne.Size) fyne.CanvasObject {
	r := &minSizeRect{size: size}
	r.ExtendBaseWidget(r)
	return r
}

type minSizeRect struct {
	widget.BaseWidget
	size fyne.Size
}

func (m *minSizeRect) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(color.Transparent)
	return widget.NewSimpleRenderer(r)
}

func (m *minSizeRect) MinSize() fyne.Size { return m.size }

// buildNumpad returns the numpad widget and a refresh function that hides
// buttons for digits that are fully used up (all 9 placed on the board).
// Exhausted digit slots stay blank — the grid never collapses or reflows.
func buildNumpad(bw *BoardWidget) (fyne.CanvasObject, func()) {
	// Keep references to digit buttons so we can show/hide them.
	digitBtns := make([]*widget.Button, 10) // index 1-9 used; 0 unused

	cells := make([]fyne.CanvasObject, 0, 10)
	for d := 1; d <= 9; d++ {
		d := d // capture
		btn := widget.NewButton(fmt.Sprintf("%d", d), func() {
			bw.PlaceDigit(d)
		})
		digitBtns[d] = btn
		cells = append(cells, tallSlot(btn))
	}
	clearBtn := widget.NewButtonWithIcon("", theme.DeleteIcon(), func() {
		bw.ClearSelected()
	})
	cells = append(cells, tallSlot(clearBtn))

	grid := container.New(layout.NewGridLayoutWithColumns(5), cells...)

	// refresh updates digit buttons (count-exhausted) and the clear button
	// (only visible when a cell is selected).
	refresh := func() {
		counts := bw.DigitCounts()
		for d := 1; d <= 9; d++ {
			if counts[d] >= sudoku.BoardSize {
				digitBtns[d].Hide()
			} else {
				digitBtns[d].Show()
			}
		}
		if bw.selRow >= 0 && !bw.solving {
			clearBtn.Show()
		} else {
			clearBtn.Hide()
		}
	}
	// Apply initial state.
	refresh()

	// Also refresh when the selection changes (tap or new game).
	bw.OnSelectionChanged = refresh

	return grid, refresh
}

// ---- New-game dialog ---------------------------------------------------

// showNewGameDialog presents Easy / Medium / Hard choices and generates a
// fresh random puzzle for the selected difficulty.
func showNewGameDialog(w fyne.Window, bw *BoardWidget) {
	type option struct {
		label string
		diff  sudoku.Difficulty
	}
	options := []option{
		{"Very Easy", sudoku.VeryEasy},
		{"Easy", sudoku.Easy},
		{"Medium", sudoku.Medium},
		{"Hard", sudoku.Hard},
		{"Very Hard", sudoku.VeryHard},
	}

	// Build one button per difficulty level.
	var dlg dialog.Dialog
	buttons := make([]fyne.CanvasObject, len(options))
	for i, o := range options {
		o := o // capture
		btn := widget.NewButton(o.label, func() {
			dlg.Hide()
			puzzle := sudoku.GenerateBoard(o.diff)
			bw.UpdateBoard(puzzle) //nolint:errcheck
		})
		buttons[i] = btn
	}

	content := container.NewVBox(
		widget.NewLabel("Choose difficulty:"),
		container.NewGridWithColumns(3, buttons[:3]...),
		container.NewGridWithColumns(2, buttons[3:]...),
	)
	dlg = dialog.NewCustom("New Game", "Cancel", content, w)
	dlg.Show()
}

// ---- Solve dialog ------------------------------------------------------

// showSolveConfirm asks the user to confirm before auto-solving.
// It solves the puzzle in memory first, then places each answer cell one by
// one with a short delay so the player can watch the board fill in.
func showSolveConfirm(w fyne.Window, bw *BoardWidget) {
	dialog.ShowConfirm(
		"Auto-Solve",
		"Solve the puzzle automatically?",
		func(ok bool) {
			if !ok {
				return
			}

			// Snapshot the current board and solve a copy in memory.
			snapshot := bw.board.GetBoard()
			solved := snapshot // will be overwritten by solver
			count := 0
			sudoku.SolveMatrix(&solved, &count)

			if count == 0 {
				fyne.Do(func() {
					dialog.ShowInformation("No Solution", "This puzzle has no valid solution.", w)
				})
				return
			}

			// Collect only the cells that were empty (need to be filled).
			type cell struct{ r, c, v int }
			var toPlace []cell
			for r := 0; r < sudoku.BoardSize; r++ {
				for c := 0; c < sudoku.BoardSize; c++ {
					if snapshot[r][c] == 0 {
						toPlace = append(toPlace, cell{r, c, solved[r][c]})
					}
				}
			}

			// Shuffle so numbers appear in a random order.
			rand.Shuffle(len(toPlace), func(i, j int) {
				toPlace[i], toPlace[j] = toPlace[j], toPlace[i]
			})

			// Place each answer on the main thread with a short delay between.
			go func() {
				fyne.Do(func() {
					bw.solving = true
					if bw.OnSelectionChanged != nil {
						bw.OnSelectionChanged()
					}
				})
				for _, cell := range toPlace {
					cell := cell
					fyne.Do(func() {
						bw.selRow = cell.r
						bw.selCol = cell.c
						bw.PlaceDigit(cell.v)
					})
					time.Sleep(80 * time.Millisecond)
				}
				// Final cleanup — deselect, clear solving flag, refresh numpad, wave.
				fyne.Do(func() {
					bw.solving = false
					bw.selRow = -1
					bw.selCol = -1
					bw.Refresh()
					bw.notifyDigitCounts()
					if bw.OnSelectionChanged != nil {
						bw.OnSelectionChanged()
					}
					bw.WaveAllCells()
					// Puzzle is now complete — stop the timer.
					if bw.OnSolved != nil {
						bw.OnSolved()
					}
				})
			}()
		},
		w,
	)
}

// ---- Starter puzzle ----------------------------------------------------

// starterPuzzle returns a well-known easy Sudoku puzzle (0 = empty).
func starterPuzzle() [sudoku.BoardSize][sudoku.BoardSize]int {
	return [sudoku.BoardSize][sudoku.BoardSize]int{
		{5, 3, 0, 0, 7, 0, 0, 0, 0},
		{6, 0, 0, 1, 9, 5, 0, 0, 0},
		{0, 9, 8, 0, 0, 0, 0, 6, 0},
		{8, 0, 0, 0, 6, 0, 0, 0, 3},
		{4, 0, 0, 8, 0, 3, 0, 0, 1},
		{7, 0, 0, 0, 2, 0, 0, 0, 6},
		{0, 6, 0, 0, 0, 0, 2, 8, 0},
		{0, 0, 0, 4, 1, 9, 0, 0, 5},
		{0, 0, 0, 0, 8, 0, 0, 7, 9},
	}
}
