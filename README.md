# Sudoku

A fully-featured Sudoku game built with **Go** and **[Fyne](https://fyne.io/)**, targeting **iOS** and **Android** from a single codebase.

For some reason there isn't a simple free Sudoku app so here is one. I added all the features I would think you would actually care about.

---

## Features

### Gameplay
- **9×9 Sudoku board** with a fully custom renderer — no off-the-shelf grid widgets
- **Five difficulty levels:** Very Easy, Easy, Medium, Hard, Very Hard
- Randomly generated puzzles with a guaranteed unique solution at every difficulty
- Tap any cell, then tap a number on the numpad to place it
- **Conflict detection** — wrong numbers are written in red and highlighted immediately
- **Same-value highlighting** — all cells matching the selected digit are shaded
- **Row/column/box highlighting** — the selected cell's row, column and 3×3 box are tinted
- **Delete button** — clears the selected cell (auto-hides when no cell is selected)
- **Number pad auto-hides** exhausted digits once all 9 of a given number are placed

### Undo
- **Undo button** in the toolbar reverses the last player action (place or clear)
- Undo stack is preserved across app restarts — history is never lost mid-game
- Button is disabled when there is nothing to undo and re-enables automatically

### Hints
- **Hint button** reveals one randomly chosen correct cell per tap
- Each game starts with **3 hints**; the button shows the remaining count and disables at 0
- Hints are undoable and the remaining count survives app restarts

### Animations
- **Error flash** — red fade-out animation on the cell when a conflicting digit is entered
- **Completion wave** — a ripple animation sweeps across any row, column, or 3×3 box the moment it is completed correctly
- **Solve wave** — a full-board diagonal wave animation plays when the puzzle is completed (manually or via auto-solve)

### Stats Row
- **Mistakes counter** — tracks how many conflicting digits have been placed this game; turns red when errors > 0
- **Timer** — counts up from 0:00, stops when the puzzle is solved, pauses when the app is backgrounded, resets on new game

### Statistics Screen
Tap the **ⓘ** button in the toolbar to open the stats screen:
- **Wins** and **Best Time** per difficulty level
- Stats persist across app restarts (stored in system preferences)
- **Reset Stats** button (confirm-gated) wipes all records

### Auto-Solve
- Confirm dialog before solving
- Solver fills in the board cell-by-cell with a short delay so you can watch it work
- Full-board wave plays at the end and the timer stops
- Auto-solved games are **not** counted in stats

### Session Persistence
- The current board, timer, difficulty, mistake count, hint count, and full undo history are **automatically saved** after every move
- Closing or force-killing the app and reopening it resumes exactly where you left off
- Starting a new game or completing a puzzle clears the saved session

### Dark Mode
- 🌙 / ☀️ toggle button in the toolbar switches between light and dark palettes instantly
- Defaults to the OS light/dark preference on launch
- Both the board and all Fyne widgets (buttons, dialogs, toolbar) respect the selected theme

### New Game Dialog
Tap **New Game** to choose from five difficulty levels:

| Level | Approx. givens |
|---|---|
| Very Easy | ~46 |
| Easy | ~36 |
| Medium | ~28 |
| Hard | ~22 |
| Very Hard | ~17 (minimum for unique solution) |

---

## Tech Stack

| | |
|---|---|
| Language | Go 1.26 |
| UI framework | [Fyne v2](https://fyne.io/) |
| Board rendering | Fully custom `fyne.Widget` + `fyne.WidgetRenderer` |
| Puzzle generation | Backtracking solver + uniqueness-preserving cell removal |
| Persistence | `fyne.Preferences` (NSUserDefaults on iOS, SharedPreferences on Android) |
| Platforms | iOS, Android |

---

## Project Structure

```
.
├── main.go                  # Entry point — calls ui.Run()
├── FyneApp.toml             # Fyne app metadata (name, ID, version, icon)
├── Icon.png                 # 1024×1024 app icon
├── deploy_iphone.sh         # Build + sign + install to a connected iPhone
├── deploy_android.sh        # Build + install to a connected Android device
├── sudoku/
│   ├── engine.go            # Board type, PlaceNumber, ClearCell, Undo, Hint, etc.
│   ├── generator.go         # Random puzzle generation + backtracking solver
│   ├── validator.go         # Row/column/box conflict checks
│   ├── errors.go            # Sentinel error values
│   └── engine_test.go       # Unit tests for the engine
└── ui/
    ├── app.go               # Fyne app setup, toolbar, numpad, timer, dark mode, session restore
    ├── board_widget.go      # Custom BoardWidget renderer, animations, input handling
    ├── stats.go             # Win/best-time persistence and stats dialog
    └── session.go           # Save/load game session (board, timer, undo stack, hints, mistakes)
```

---

## Building & Deploying

### Prerequisites
- Go 1.21+
- [Fyne CLI](https://docs.fyne.io/started/cli.html): `go install fyne.io/tools/cmd/fyne@latest`

### iOS (requires macOS + Apple Developer account)
```sh
./deploy_iphone.sh
```
Builds for `arm64`, injects a provisioning profile, re-signs, and installs via `xcrun devicectl`.

### Android (requires Android SDK + NDK)
```sh
./deploy_android.sh
```
Builds an APK via `fyne package --target android` and installs via `adb`. The APK is deleted automatically after install.

### Run on Desktop (for development)
```sh
go run .
```

---

## Running Tests

```sh
go test ./sudoku/...
```

---

## License

MIT
