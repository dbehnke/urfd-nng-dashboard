# URFD NNG Dashboard

A modern, real-time dashboard for the [Universal Reflector (urfd)](https://github.com/dbehnke/urfd), built using **Go**, **Vue 3**, and **NNG** (Nanomsg Next Gen).

This dashboard serves as a high-performance alternative to the legacy PHP-based dashboard, consuming events directly from `urfd`'s NNG publisher socket to provide instant updates without polling.

## Features

- **Real-Time Updates**: Uses Websockets to push NNG events (Hearings, Connections) directly to the browser.
- **Modern UI**: Built with [Vue 3](https://vuejs.org/) and [Tailwind CSS 4](https://tailwindcss.com/), offering a responsive and clean design.
- **Dark Mode**: Native support for Light, Dark, and System themes.
- **Activity Log**: "Last Heard" list with live duration tracking, session de-duplication, and protocol information.
- **Resource Efficient**: Backend written in Go with a lightweight SQLite (WAL mode) database for history.
- **Deployment Ready**: Includes Docker Compose setup and Systemd service files/scripts.

## Architecture

The system consists of two main components bundled into a single binary:

1. **Backend (Go)**:
    - **NNG Subscriber**: Connects to `urfd` (default `tcp://127.0.0.1:5555`) to listen for JSON events (`hearing`, `state`, etc.).
    - **Store**: Persists call history and node states to a local SQLite database using GORM.
    - **Websocket Hub**: Broadcasts live events to connected frontend clients.
    - **HTTP Server**: Serves the embedded Vue frontend and API endpoints.

2. **Frontend (Vue 3 + Vite)**:
    - **State Management**: Uses Pinia to track active sessions, nodes, and configurations.
    - **Reactivity**: displaying active talkers with real-time "time since" and duration ticking.

## Installation

### Prerequisites

- A running instance of `urfd` with NNG publishing enabled (usually on port 5555).
- Go 1.25+ (for building from source).
- Bun or Node.js (for building the frontend).

### Building from Source

The project uses [Task](https://taskfile.dev/) for automation.

1. **Clone the repository**:

   ```bash
   git clone https://github.com/dbehnke/urfd-nng-dashboard.git
   cd urfd-nng-dashboard
   ```

2. **Build the project**:

   ```bash
   task build
   ```

   This produces `urfd-dashboard` and `urfd-simulator` in the root directory.

### Configuration

The dashboard is configured via `config.yaml` (default) or command-line flags.

**Example `config.yaml`**:

```yaml
server:
  addr: ":8080"
  nng_url: "tcp://127.0.0.1:5555"
  db_path: "data/dashboard.db"

reflector:
  name: "My Reflector"
```

A full example is available in `examples/config.yaml`.

Run with specific config:

```bash
./urfd-dashboard --config my-config.yaml
```

## Deployment

### Docker Compose (Recommended)

```bash
cd deploy
# Optionally create a config.yaml based on examples/config.yaml
docker-compose up -d
```

### Systemd

A sample service unit is available in `deploy/systemd/urfd-dashboard.service`.

## Development

### Simulator

To test without a live reflector:

```bash
# Build
task build-simulator

# Run (defaults to publishing on tcp://127.0.0.1:5555)
./urfd-simulator
```

### Hot Reload

For frontend development with hot reload:

1. Start the backend (to serve the API/Websocket):

    ```bash
    go run cmd/dashboard/main.go
    ```

2. Start the frontend dev server (in `web/`):

    ```bash
    cd web
    bun run dev
    ```

## License

MIT
