# Teno Voice

Teno Voice is a REST API that connects to a Discord voice server, receives audio from other clients in the Discord server, and sends its own audio to the server.

## Table of Contents

- [Installation](#installation)
- [Usage](#usage)
- [Configuration](#configuration)
- [Development](#development)
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
DISCORD_TOKEN=<your-discord-bot-token>
```

Replace `<your-discord-bot-token>` with the token for the Teno Discord bot.

## Development

To work on the Teno Voice project, you'll need to have Go installed on your system. Use the following command to run the application in development mode:

```
go run .
```

## Contributing

Contributions are internal to DeabLabs for now.
