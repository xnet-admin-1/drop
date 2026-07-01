# Drop

File drop + web terminal + joint user/agent browser. Single binary, no dependencies.

## Features

- File upload/download with drag & drop
- **Tabbed web terminal** — multiple shell sessions, each with its own PTY
  - Open/close/clear tabs, per-session reconnect
  - Persistent sessions survive page reloads (ring buffer replay)
- **Touch selection** (Acode-style) — long-press to select, drag handles, Copy/Paste/All menu
- **Mobile optimized** — extra keys bar (ESC, TAB, CTRL, ALT, arrows, HOME, END, DEL), smooth touch scrolling, floating keyboard tracking
- **Joint user/agent browser** — Headless Chrome with real-time streaming, controllable by both user and AI agent
- PWA installable
- Basic auth
- Single binary, no dependencies (except Chrome, see below)

## Browser

The joint browser feature requires Chrome/Chromium to be installed on the system.

### Setup

Run the setup script to install Chromium:

```bash
make setup-browser
# or directly
./scripts/setup-browser.sh
```

This will detect your OS and install the appropriate browser package.

### Manual Installation

If the setup script doesn't work for your system:

| OS | Command |
|----|---------|
| Ubuntu/Debian | `apt-get install chromium-browser` |
| Alpine | `apk add chromium` |
| macOS | `brew install chromium` |

Make sure `google-chrome`, `chromium-browser`, or `chromium` is in your PATH.

## Build

### Native

```bash
go build -o drop .
# or
make build
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

## Browser Features

The joint browser allows both user and AI agent to interact with the same browser tab:

- **Real-time streaming** — 15fps screenshot updates via WebSocket
- **User controls** — Click, type, scroll, back/forward in the browser view
- **Agent controls** — AI agent can navigate, click, type via MCP tools
- **Shared state** — Both see the same page content

Access the browser at `/browser_view.html`.