# tasks-mcp

MCP server for Google Tasks. Runs over stdio using JSON-RPC 2.0.

## Features

- **list_task_lists** — show all task lists
- **list_tasks** — tasks from a list (with optional completed)
- **create_task** — new task with optional due date/time and notes
- **update_task** — modify task fields
- **complete_task** — mark as done
- **delete_task** — remove a task

## Requirements

- Go 1.24+
- Google Cloud project with Tasks API enabled
- OAuth2 desktop app credentials

## Setup

### 1. Google Credentials

Follow [docs/google-oauth-setup.md](docs/google-oauth-setup.md) to create OAuth credentials and authorize.

### 2. Install

```bash
go install github.com/cherya/google-tasks-mcp@latest
```

Or build from source:

```bash
git clone https://github.com/cherya/google-tasks-mcp.git
cd google-tasks-mcp
go build -o google-tasks-mcp .
```

### 3. Authorize

```bash
# Get the authorization URL
GOOGLE_OAUTH_CREDENTIALS=/path/to/oauth-client.json google-tasks-mcp --auth

# Open the URL in a browser, authorize, copy the code

# Exchange the code for a token
GOOGLE_OAUTH_CREDENTIALS=/path/to/oauth-client.json google-tasks-mcp --token <CODE>
```

### 4. Environment Variables

- `GOOGLE_OAUTH_CREDENTIALS` — path to OAuth client JSON (required)
- `GOOGLE_TOKEN_FILE` — path to token storage (optional, defaults to `tasks-token.json` next to credentials)
- `TIMEZONE` — IANA timezone for due dates, e.g. `Asia/Tbilisi` (optional, defaults to `UTC`)

## Usage with Claude Desktop

Add to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "google-tasks": {
      "command": "google-tasks-mcp",
      "env": {
        "GOOGLE_OAUTH_CREDENTIALS": "/path/to/oauth-client.json",
        "GOOGLE_TOKEN_FILE": "/path/to/tasks-token.json",
        "TIMEZONE": "Asia/Tbilisi"
      }
    }
  }
}
```

## License

[MIT](LICENSE)
