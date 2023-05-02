# Teno Voice

Teno Voice is a REST API that connects to a Discord voice server, receives audio from other clients in the Discord server, and sends its own audio to the server.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [Development](#development)
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

1. Before running the application, make sure to [configure](#configuration) it properly.
2. Run the application:

   ```
   ./teno-voice
   ```

3. To interact with the REST API, use a tool like [curl](https://curl.se/) or [Postman](https://www.postman.com/).

## Configuration

To configure Teno Voice, create a `.env` file in the project root and set the following environment variables:

```
TOKEN=<your-discord-bot-token>
```

Replace `<your-discord-bot-token>` with the token for the Teno Discord bot.

You will also need to setup and configure ngrok in order to run locally with wss and https support

Follow the instructions here: https://ngrok.com/download

Once you have ngrok installed, with your API key configured, run the following command:

```
ngrok http 8080
```

This will create a tunnel to your local machine on port 8080.

## Development

To work on the Teno Voice project, you'll need to have Go installed on your system. Use the following command to run the application in development mode:

```
go run .
```

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
