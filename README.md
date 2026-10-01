# Plips

Tiny creatures made of liquid pixels, with little brains that learn. They live on your desktop,
eat the coloured drops you give them, and pick up habits from how you treat them.

Early days. See `CLAUDE.md` for the plan and where things stand.

## Build (Windows exe, from any OS)

```
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui" -o plips.exe .
```
