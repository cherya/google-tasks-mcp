# Google OAuth Setup

This guide walks through creating OAuth2 credentials for the Google Tasks API.

## Step 1: Create a Google Cloud Project

1. Open [Google Cloud Console](https://console.cloud.google.com/)
2. Click the project dropdown → **New Project**
3. Enter a name (e.g., `tasks-mcp`) → **Create**

## Step 2: Enable the Tasks API

1. Go to **APIs & Services** → **Library**
2. Search for **Google Tasks API**
3. Click **Enable**

## Step 3: Configure OAuth Consent Screen

1. Go to **APIs & Services** → **OAuth consent screen**
2. Select **External** → **Create**
3. Fill in the required fields (app name, user support email, developer email)
4. Click **Save and Continue** through the remaining steps
5. Under **Test users**, add your Google account email
6. Click **Save**

> Note: While in "Testing" mode, only added test users can authorize. This is fine for personal use.

## Step 4: Create OAuth Client ID

1. Go to **APIs & Services** → **Credentials**
2. Click **Create Credentials** → **OAuth client ID**
3. Application type: **Desktop app**
4. Enter a name (e.g., `tasks-mcp`) → **Create**
5. Click **Download JSON** and save the file securely

The file looks like:

```json
{
  "installed": {
    "client_id": "123456789.apps.googleusercontent.com",
    "client_secret": "...",
    "auth_uri": "https://accounts.google.com/o/oauth2/auth",
    "token_uri": "https://oauth2.googleapis.com/token",
    "redirect_uris": ["urn:ietf:wg:oauth:2.0:oob", "http://localhost"]
  }
}
```

## Step 5: Authorize

Run the auth flow to get an access token:

```
# Get the authorization URL
GOOGLE_OAUTH_CREDENTIALS=/path/to/oauth-client.json ./tasks-mcp --auth

# Open the printed URL in a browser, authorize, and copy the code

# Exchange the code for a token
GOOGLE_OAUTH_CREDENTIALS=/path/to/oauth-client.json ./tasks-mcp --token <CODE>
```

The token is saved to `tasks-token.json` next to the credentials file by default. Override with `GOOGLE_TOKEN_FILE` env var.

## Troubleshooting

**"Access blocked: This app's request is invalid"** — the OAuth consent screen is not configured, or the redirect URI is missing from the client config.

**"Error 403: access_denied"** — your Google account is not added as a test user in the OAuth consent screen settings.

**"Token expired"** — the server auto-refreshes tokens on startup. If the refresh token itself is revoked, re-run `--auth` and `--token`.
