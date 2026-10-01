# Plips: notes for Claude

Plips are tiny creatures made of simulated liquid, drawn as chunky pixels, with small learning
brains. The goal is a Windows desktop app where a pair of them lives on top of the user's
desktop, so they can come home from work and see what the plips got up to.

Inspired by **Creatures** (1996, Steve Grand): Norns with neural-net brains, biochemistry
and genetics, who learn words for objects. The user loved two ideas from it: teaching them
things, and appearance changing based on what happens to them (like Wobbledogs).

## How to work with this user

- Casual and exploratory. They figure out what they want by seeing it. Get something on screen
  fast and iterate, rather than writing long plans. They've said "you're thinking too far
  ahead": keep suggestions to the next step or two.
- They often add requests mid-task. Fold them in without fuss.
- Swearing is just how they talk.

## Decisions so far

- **Name:** the creatures are *plips* (the sound a drop makes).
- **Look:** pixel art with real liquid physics inside the pixels. Bodies are a few dozen to a few
  hundred fluid particles (double-density relaxation, Clavet et al. 2005), rendered into a
  low-res pixel buffer with an edge highlight and outline. Small on screen.
- **Platform:** a native Windows `.exe`, not a browser. Go + [Ebitengine](https://ebitengine.org)
  cross-compiles without cgo: `GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o plips.exe .`
  Keep the simulation in `sim/` free of graphics imports so it can run headless on Linux
  (render frames to PNG to check the look).
- **Base form first:** the user isn't settled on the look. Build a **variation gallery**: a grid
  of about 12 baby plips with random body genes (size, layout/lobes, aspect, softness, tautness,
  pack/pressure, jiggle, breathing, nucleus, colour, speckle, spots, rim, translucency, eyes,
  pupils, gait). Click one to refill the grid with its mutants. Save favourites.
- **Food is just colours for now** (red, orange, yellow, green, blue, violet, white, black).
  No named substances. If real substances come later, their effects must match what they are.
  Each plip has per-colour *taste* genes, so preferences differ between individuals.
- **Mouse controls:**
  - left tap = one drop
  - hold left = a straight stream falling from the cursor
  - right click = a splash (a blob of about 30 particles)
  - on a plip: left = pet (reward), right = poke (punish, splashes, some liquid breaks off and
    can be re-eaten)
- **Two plips with brains that interact.** Brain plan (Creatures-lite):
  - Drives: hunger, pain, boredom, loneliness, tiredness.
  - Attention targets: each colour puddle, the other plip, the hand (cursor), nothing.
  - Actions: approach, eat, retreat, play, rest, hop.
  - Network: a linear layer over [drives, target one-hot, drive x target, bias] scores each
    (target, action) pair; pick by softmax using the genome's temperature.
  - Learning: reward = drop in weighted drives over the action, plus pets minus pokes;
    `w += lr * reward * features`.
  - Eating only absorbs liquid while the "eat" action is running. Eating the other plip steals
    particles and hurts it.
- **Modes:** gallery, room (a window with a little pixel room: wall, window, shelf, table,
  floorboards; shelf and table are one-way platforms), and **desktop overlay**:
  - transparent, borderless, always-on-top, covering the monitor work area, with the floor on
    top of the taskbar
  - click-through except when the cursor is over a plip or liquid pixel, or while Ctrl+Alt is
    held (poll with win32 `GetCursorPos` / `GetAsyncKeyState`)
  - global hotkeys Ctrl+Alt+1/2/3 to switch modes, Ctrl+Alt+Q to quit
  - should be configurable in a settings file
- **Persistence:** autosave genomes, brains and drives (e.g. `%APPDATA%/Plips/`) every minute and
  on exit, so the plips are still themselves next time.
- **Performance:** this runs alongside games (the user builds an s&box game that must hold
  60 fps). Keep the sim small and single-threaded, and set the process to below-normal priority
  on Windows.

## Later ideas (not now)

- Teaching words: point at a thing and type its name, then type the word and the plip goes to it.
  On the desktop the "things" could be real windows and icons.
- Standing on top of open windows (EnumWindows) as platforms.
- A diary of what happened while the user was away.
- Breeding (`sim.Cross` already exists).

## State of the code (v0.1)

Layout:
- `main.go`: the Ebitengine app. Handles modes (F1/F2/F3, or Ctrl+Alt+1/2/3 from anywhere), window
  setup per mode, desktop click-through, autosave and Ctrl+Alt+Q to quit.
- `gallery.go`: a 4x3 grid of brainless baby plips. Click = refill with mutants of that one,
  right-click = star (saved to `favourites.json`), Space = all new.
- `pets.go`: the live pair with brains, used by both the room and desktop modes. Handles mouse
  controls, the info panel (Tab or Ctrl+Alt+H) and `NewPair` (hatches from the last two starred).
- `store.go`: JSON files in `%APPDATA%\Plips\` (`settings.json`, `pets.json`, `favourites.json`).
- `platform_windows.go`: win32 calls (global cursor, global keys, taskbar work area,
  below-normal priority). `platform_other.go` holds stubs.
- `sim/`: world physics (`world.go`), genes (`genome.go`), body and behaviour (`plip.go`),
  brain (`brain.go`), pixel renderer (`render.go`), room art and platforms (`room.go`).
- `cmd/shot`: a headless run that writes `gallery.png`, `room-mid.png` and `room.png` and prints
  brain stats: `go run ./cmd/shot -out shots` (pass a negative `-seed` to get a gallery of one
  family). Use it to check looks and behaviour from Linux.
- `prototype/slosh-tank.html`: the original browser toy.

Build: `GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o dist/plips.exe .`
The `main` package only builds for Windows from Linux. Ebitengine on Linux needs cgo and X11
headers, which this container doesn't have. `go vet` works with `GOOS=windows`.

Known gaps and next ideas:
- None of the window modes have been run on real Windows yet; the desktop overlay especially
  needs checking (transparency, click-through toggling, taskbar floor, DPI scaling).
- The gallery reads mostly as blobs. Lobed bodies (chain/stack/clump) only show as faint
  creases, so shapes need to be more distinct.
- Spilled liquid spreads into thin films on the floor.
- No offline time: the plips only live while the app is running.
- The info panel uses Ebitengine's debug font. Swap in a proper pixel font.
