# Plips

Tiny creatures made of liquid pixels, with little brains that learn. They live on your desktop,
eat the coloured drops you give them, and pick up habits from how you treat them.

Early days. See `CLAUDE.md` for the plan and where things stand.

## Controls

Everything is on-screen buttons. In the room: tap = a drop, hold = a stream, right-click = a
splash; on a plip, left-click pets it and right-click pokes it.

Pets, favourites and settings save to `%APPDATA%\Plips\`.

## Build (Windows exe, from any OS)

```
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o plips.exe .
```
