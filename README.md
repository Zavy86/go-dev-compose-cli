# go-dev-compose-cli

A terminal-based TUI (Text User Interface) for managing a Docker Compose environment.

This project is a **proof of concept (POC) created with Gemini Flash 6** for educational purposes. Its goal is to learn
**Go from scratch** and become familiar with the **Docker SDK for Go**, focusing on Compose file parsing, container
management, and log streaming.

## Features

- automatically searches for a `compose.yml` file;
- parses the Compose project and displays its services;
- connects to the Docker Engine through the Docker SDK for Go;
- displays container statuses with automatic refresh;
- starts and stops the entire Compose project;
- starts, stops, and restarts an individual container;
- streams logs for the whole project or the selected service;
- provides an interactive terminal interface built with `tview` and `tcell`.

## Prerequisites

- Go `1.26.5`, or a compatible version supported by the module;
- a valid Compose file, such as the one included in this repository.
- Docker Desktop or a running, reachable Docker Engine;

## Getting started

Clone the repository, download the dependencies, and run the TUI:

```bash
git clone https://github.com/Zavy86/go-dev-compose-cli.git
cd go-dev-compose-cli
go mod download
go run ./cmd/tui
```

At startup, the application searches for `compose.yml` in the current directory, loads its configuration, and displays
the available services. You can validate the included Compose file before starting the application:

```bash
docker compose -f compose.yml config
go run ./cmd/tui
```

## Development commands

Run the application without creating a binary:

```bash
go run ./cmd/tui
```

Build the application into `dev-cli`:

```bash
go build -o dev-cli ./cmd/tui
```

Run the compiled binary:

```bash
./dev-cli
```

Format the Go source files:

```bash
gofmt -w $(find cmd internal -type f -name '*.go')
```

Remove downloaded module cache entries and build cache entries when needed:

```bash
go clean -cache -modcache
```

## Keyboard controls

When the TUI is running, select a service with the arrow keys and use:

| Key | Action |
| --- | --- |
| `u` | Start the Compose project |
| `d` | Stop and remove the Compose project |
| `a` | Start all containers |
| `o` | Stop all containers |
| `s` | Start the selected service |
| `x` | Stop the selected service |
| `r` | Restart the selected service |
| `c` | Clear the log panel |

## Project structure

```text
cmd/tui/              Application entry point
internal/config/      Compose configuration discovery and parsing
internal/docker/      Docker Engine client and container operations
internal/tui/         Terminal interface and event handling
compose.yml           Example Compose environment
```

## Project purpose

The code is intentionally focused on learning and is not intended to replace the Docker Compose CLI in production
environments. It can be used as a starting point for studying:

- Go project organization;
- module and dependency management;
- contexts and goroutines;
- integration with external APIs and SDKs;
- building TUIs;
- asynchronous status and log handling.

## Contributing

Feel free to contribute to this repository by opening an issue or submitting a pull request.
