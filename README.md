# Teno Voice

Teno Voice is a REST API that connects to a Discord voice server, receives audio from other clients in the Discord server, and sends its own audio to the server.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Structure](#structure)
- [Contributing](#contributing)

## Installation

1. Make sure you have [Go](https://golang.org/dl/) installed on your system.
2. Clone this repository:

   ```
   git clone https://github.com/deablabs/teno-voice.git
   ```

3. Navigate to the project directory:

   ```
   cd teno-voice
   ```

4. Build the application:

   ```
   go build
   ```

## Usage

Because of https restrictions using the deepgram API, teno-voice must be run on a server with valid SSL certificates.

We use <https://fly.io> but you could configure a Dockerfile with certs or use a service like <https://letsencrypt.org/> to get valid certs locally.

To initially launch on fly.io, you must have the fly CLI installed and be logged in.

`flyctl launch`

To deploy new changes,

`flyctl deploy -a app-name`

Github actions are configured to deploy on push to main and production, using app names `teno-voice-staging` and `teno-voice` respectively.

## Structure

### Teno Voice Project Structure

This document describes the suggested file and directory structure for the Teno Voice project.

```
teno-voice/
├── main.go
├── internal/
│   ├── config/
│   │   └── config.go
│   ├── database/
│   │   └── database.go
│   ├── discord/
│   │   └── discord.go
│   ├── handlers/
│   │   ├── join.go
│   │   └── leave.go
│   ├── middleware/
│   │   └── middleware.go
│   ├── llm/
│   │   └── llm.go
│   ├── textToVoice/
│   │   └── textToVoice.go
│   └── voiceToText/
│       └── voiceToText.go
├── pkg/
│   ├── models/
│   │   └── models.go
│   └── utils/
│       └── utils.go
├── .env
├── .gitignore
├── go.mod
├── go.sum
└── README.md
```

### Directory Structure

- `main.go`: This is the main entry point for the application.
- `internal/`: This directory contains the private code of the application, organized by functionality.
  - `config/`: Contains configuration-related code, such as loading environment variables.
  - `database/`: Contains code related to database connections and operations.
  - `discord/`: Contains code related to Discord API interactions.
  - `handlers/`: Contains HTTP handler functions, such as `joinVoiceCall` and `leaveVoiceCall`.
  - `middleware/`: Contains any middleware functions needed for the HTTP server.
  - `llm/`: Contains code for handling LLM interactions.
  - `voice/`: Contains code for handling voice processing.
- `pkg/`: This directory contains code that can be reused by other projects, such as utility functions or data models.
  - `models/`: Contains data models and structs that may be shared across packages.
  - `utils/`: Contains utility functions and helper code.
- `.env`: Contains environment variables for the application.
- `.gitignore`: Lists files and directories that should be ignored by Git.
- `go.mod` and `go.sum`: Contain Go module information and dependencies.
- `README.md`: Provides documentation for the application.

## Contributing

Contributions are internal to DeabLabs for now.
