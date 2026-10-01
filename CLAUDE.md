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

## State of the code

- `prototype/slosh-tank.html`: the first browser toy (one blob, drop colours, poke). It's a
  reference for the physics and rendering, which work and look right there.
- `sim/world.go`: particle world, room collision, absorption and stealing. **Work in progress:**
  it still has the old named liquids (`Mats`), fixed room size constants and the old genome field
  names (`StartMass`, `Base`). Make it match the plan: colours palette, a world size set per mode,
  and per-particle lobe/role.
- `sim/genome.go`: body and mind genes, `RandomGenome`, `Mutate`, `Cross`, `Describe`. Matches
  the plan.
- Not written yet: `sim/critter.go` (body control with lobes/aspect/softness, eyes, behaviour),
  `sim/brain.go`, `sim/render.go` (RGBA buffer with transparent mode), the room background, the
  headless `cmd/shot` PNG tool, and the Ebitengine `main` package with the modes.
- It doesn't compile yet. Next step: get the gallery on screen and send the user a Windows exe.
