# Plips

Tiny creatures made of liquid pixels, with little brains that learn. They live on your desktop,
eat the coloured drops you give them, and pick up habits from how you treat them.

Early days. See `CLAUDE.md` for the plan and where things stand.

## Controls

| | |
|---|---|
| F1 / F2 / F3 (or Ctrl+Alt+1/2/3 anywhere) | gallery / room / desktop |
| Gallery: click, right-click, Space | variations of this one, star it, all new |
| Tap left | one drop |
| Hold left | a stream |
| Right click | a splash |
| Left / right on a plip | pet / poke |
| 1-8 or mouse wheel | food colour |
| Tab (or Ctrl+Alt+H) | info panel |
| N | hatch a new pair (from your two latest stars) |
| Ctrl+Alt (desktop mode) | hold to feed anywhere; otherwise clicks go through to your desktop |
| Ctrl+Alt+Q | quit (the desktop overlay has no close button) |

Pets, favourites and settings save to `%APPDATA%\Plips\`.

## Build (Windows exe, from any OS)

```
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o plips.exe .
```
