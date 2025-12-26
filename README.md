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

The project uses [Task](https://taskfile.dev/) for build automation.

1. **Clone the repository**:

    ```bash
    git clone https://github.com/dbehnke/urfd-nng-dashboard.git
    cd urfd-nng-dashboard
    ```

2. **Build the project**:
    This command builds the frontend assets and embeds them into the Go binary.

    ```bash
    task build
    ```

    The output binary `urfd-dashboard` will be in the root directory.

### Configuration

The dashboard accepts the following command-line flags:

- `--nng-url`: The NNG URL to subscribe to (default: `tcp://127.0.0.1:5555`).
- `--db`: Path to the SQLite database file (default: `data/dashboard.db`).
- `--listen`: HTTP listen address (default: `:8080`).

## Running

### Manual

```bash
./urfd-dashboard --nng-url tcp://localhost:5555 --listen :8080
```

### Docker Compose

A `docker-compose.yml` is provided in the `deploy/` directory.

```bash
cd deploy
docker-compose up -d
```

### Systemd

A sample service unit is provided in `deploy/systemd/urfd-dashboard.service`.

1. Copy the binary to `/usr/local/bin/urfd-dashboard`.
2. Copy the service file to `/etc/systemd/system/`.
3. Enable and start the service:

    ```bash
    systemctl enable --now urfd-dashboard
    ```

## Development

### Backend Simulator

To test the dashboard without a live reflector, use the included simulator:

```bash
go run cmd/simulator/main.go --url tcp://127.0.0.1:5555
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
