# Drop

File drop + web terminal. Single binary, no dependencies.

## Features

- File upload/download with drag & drop
- **Tabbed web terminal** — multiple shell sessions, each with its own PTY
  - Open/close/clear tabs, per-session reconnect
  - Persistent sessions survive page reloads (ring buffer replay)
- **Touch selection** (Acode-style) — long-press to select, drag handles, Copy/Paste/All menu
- **Mobile optimized** — extra keys bar (ESC, TAB, CTRL, ALT, arrows, HOME, END, DEL), smooth touch scrolling, floating keyboard tracking
- PWA installable
- Basic auth
- Single binary, no dependencies

## Build

### Native

```bash
go build -o drop .
```

### Container

```bash
docker build -t drop .
```

## Run

### Native

```bash
# Set credentials first
mkdir -p ~/ai
echo 'user:pass' > ~/ai/.creds

# Run
DROP_DATA=~/ai/drop DROP_CREDS=~/ai/.creds ./drop
```

### Container

```bash
mkdir -p data/drop
echo 'user:pass' > data/.creds

docker run -d --name drop -p 9800:9800 \
  -v ./data:/data \
  -e DROP_DATA=/data/drop \
  -e DROP_CREDS=/data/.creds \
  drop
```

Or with compose:

```bash
docker compose up -d
```

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `DROP_DATA` | `~/ai/drop` | File storage directory |
| `DROP_CREDS` | `~/ai/.creds` | Credentials file (`user:pass`) |

Listens on `:9800`.
