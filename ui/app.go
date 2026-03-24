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

// startWithOffset starts the timer with a pre-existing elapsed duration,
// used when restoring a saved session.
func (gt *gameTimer) startWithOffset(offset time.Duration) {
	gt.mu.Lock()
	gt.elapsed = offset
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
		gt.elapsed += time.Since(gt.startAt) // capture remaining run before stopping
		close(gt.stopCh)
	}
}

// variantTheme wraps DefaultTheme and forces a specific light/dark variant,
// avoiding the deprecated theme.LightTheme() / theme.DarkTheme() functions.
type variantTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t variantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, t.variant)
}

func forcedTheme(variant fyne.ThemeVariant) fyne.Theme {
	return variantTheme{Theme: theme.DefaultTheme(), variant: variant}
}

// Run creates the Fyne application, builds the main window, and blocks until
// the window is closed.
func Run() {
	a := app.NewWithID("Fetzco.com-matthewfetzer-sudoku")

	w := a.NewWindow("Sudoku")

	// ---- Session restore or fresh start ------------------------------------
	board := sudoku.NewBoard()
	startElapsed := 0
	startHints := 3
	startDifficulty := sudoku.Easy
	startMistakes := 0

	if sess, ok := LoadSession(a.Preferences()); ok {
		// Restore the puzzle givens, then layer player moves on top.
		_ = board.SetBoard(sess.puzzle)
		board.RestorePlayerBoard(sess.player)
		board.RestoreUndoStack(sess.undoStack)
		startElapsed = sess.elapsed
		startHints = sess.hints
		startDifficulty = sess.difficulty
		startMistakes = sess.mistakes
	} else {
		_ = board.SetBoard(sudoku.GenerateBoard(sudoku.Easy))
	}

	bw := NewBoardWidget(board)
	bw.conflict = board.Conflicts()
	bw.errorCount = startMistakes // restore mistake count before any callbacks wire up

	// currentDifficulty tracks the active puzzle for stats recording.
	currentDifficulty := startDifficulty

	// saveCurrent is declared here so closures in the toolbar can reference it
	// before it is fully assigned after the timer is created.
	var saveCurrent func()

	// ---- Toolbar -----------------------------------------------------------

	newBtn := widget.NewButton("New Game", func() {
		showNewGameDialog(w, bw, &currentDifficulty)
	})
	newBtn.Importance = widget.HighImportance

	solveBtn := widget.NewButton("Solve", func() {
		showSolveConfirm(w, bw)
	})
	solveBtn.Importance = widget.HighImportance

	// Undo button — disabled until there is something to undo.
	undoBtn := widget.NewButtonWithIcon("", theme.ContentUndoIcon(), func() {
		bw.UndoLast()
	})
	undoBtn.Disable()

	// Hint button — restore remaining hints from session if resuming.
	hintsLeft := startHints
	var hintBtn *widget.Button
	hintBtnLabel := func() string {
		if hintsLeft > 0 {
			return fmt.Sprintf("Hint (%d)", hintsLeft)
		}
		return "No Hints"
	}
	hintBtn = widget.NewButton(hintBtnLabel(), func() {})
	if hintsLeft == 0 {
		hintBtn.Disable()
	}
	hintBtn.OnTapped = func() {
		if hintsLeft <= 0 {
			return
		}
		if bw.ApplyHint() {
			hintsLeft--
			hintBtn.SetText(hintBtnLabel())
			if hintsLeft == 0 {
				hintBtn.Disable()
			}
			// Save immediately after hint so hintsLeft is correct.
			saveCurrent()
		}
	}

	// Stats button.
	statsBtn := widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
		ShowStatsDialog(w, a.Preferences())
	})

	// Dark-mode toggle — initialised from the OS/user preference.
	// onThemeChange is assigned below once the stats labels exist.
	darkMode := a.Settings().ThemeVariant() == theme.VariantDark
	themeBtnLabel := "🌙"
	if darkMode {
		themeBtnLabel = "☀️"
	}
	var onThemeChange func()
	themeBtn := widget.NewButton(themeBtnLabel, func() {
		if onThemeChange != nil {
			onThemeChange()
		}
	})

	toolbar := container.NewHBox(
		widget.NewLabel("Sudoku"),
		layout.NewSpacer(),
		statsBtn,
		undoBtn,
		hintBtn,
		themeBtn,
		solveBtn,
		newBtn,
	)

	// ---- On-screen number pad (essential for touch/iOS) --------------------
	numpad, refreshNumpad := buildNumpad(bw)
	bw.OnDigitCountsChanged = refreshNumpad

	// ---- Error counter -----------------------------------------------------
	// Initialise text from restored session (may be non-zero on resume).
	errorLabelText := fmt.Sprintf("Mistakes: %d", startMistakes)
	errorLabelCol := colErrorLabel
	if startMistakes > 0 {
		errorLabelCol = colErrorLabelActive
	}
	errorLabel := canvas.NewText(errorLabelText, errorLabelCol)
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

	// Restore undo button state if the stack was loaded from session.
	if board.CanUndo() {
		undoBtn.Enable()
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
			a.Settings().SetTheme(forcedTheme(theme.VariantDark))
			themeBtn.SetText("☀️")
		} else {
			a.Settings().SetTheme(forcedTheme(theme.VariantLight))
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
	if startElapsed > 0 {
		gt.startWithOffset(time.Duration(startElapsed) * time.Second)
	} else {
		gt.start()
	}

	// Sync colours/widget state to the detected startup theme.
	SetDarkMode(darkMode)
	setStatColours(darkMode)

	// saveCurrent persists the ongoing game state so it survives app restarts.
	// The var is declared earlier so toolbar closures (hint button) can call it.
	saveCurrent = func() {
		gt.mu.Lock()
		var elapsed time.Duration
		if gt.paused || gt.done {
			elapsed = gt.elapsed
		} else {
			elapsed = gt.elapsed + time.Since(gt.startAt)
		}
		gt.mu.Unlock()
		SaveSession(a.Preferences(), board, currentDifficulty, int(elapsed.Seconds()), hintsLeft, bw.errorCount)
	}

	// Save on every player move so iOS SIGKILL can't lose progress.
	bw.OnBoardChanged = saveCurrent

	// Pause/resume the timer when the app backgrounds/foregrounds.
	a.Lifecycle().SetOnExitedForeground(func() {
		gt.pause()
		saveCurrent()
	})
	a.Lifecycle().SetOnEnteredForeground(func() { gt.resume() })

	// Wire undo button enable/disable to board undo stack state.
	bw.OnUndoStateChanged = func(canUndo bool) {
		if canUndo {
			undoBtn.Enable()
		} else {
			undoBtn.Disable()
		}
	}

	// Reset timer, mistakes, hints, and undo stack when a new game starts.
	// Also clear any saved session — the new game will be saved on next background.
	bw.OnNewGame = func() {
		gt.reset()
		hintsLeft = 3
		hintBtn.SetText(hintBtnLabel())
		hintBtn.Enable()
		undoBtn.Disable()
		ClearSession(a.Preferences())
	}

	// Stop timer and record win when puzzle is solved by the player.
	bw.OnSolved = func() {
		gt.stop()
		elapsed := int(gt.elapsed.Seconds())
		RecordWin(a.Preferences(), currentDifficulty, elapsed)
		ClearSession(a.Preferences())
		bw.WaveAllCells()
	}

	// Auto-solve: stop the timer but do not record in stats.
	bw.OnAutoSolved = func() {
		gt.stop()
		ClearSession(a.Preferences())
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

	// Stop the timer and save session when the window is closed (desktop/simulator).
	// On real iOS/Android the app is killed without this intercept, so the
	// background-lifecycle save (SetOnExitedForeground) covers those devices.
	w.SetCloseIntercept(func() {
		saveCurrent()
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
func showNewGameDialog(w fyne.Window, bw *BoardWidget, difficulty *sudoku.Difficulty) {
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
			*difficulty = o.diff
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
					// Puzzle is now complete via auto-solve — stop the timer only,
					// do NOT record in stats.
					if bw.OnAutoSolved != nil {
						bw.OnAutoSolved()
					}
				})
			}()
		},
		w,
	)
}
