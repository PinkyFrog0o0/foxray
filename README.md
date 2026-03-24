<p align="center">
  <img src="gmn.png" alt="FoxRay logo" width="150">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.22-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/license-Apache%202.0-blue?style=for-the-badge" alt="License">
</p>

<p align="center">
  <strong>FoxRay</strong><br>
  <em>A lightweight, non-interactive Gemini CLI fork written in Go</em>
</p>

<p align="center">
  <a href="README_ja.md">日本語</a>
</p>

## Why FoxRay?

FoxRay is your own fork-ready version of `gmn-api`, rebranded for independent maintenance and release. It keeps the fast Go-based CLI flow, API key support for Gemini API and Vertex AI Express, and MCP support, while switching the project identity to `FoxRay`.

This fork keeps compatibility where it matters:

- OAuth and MCP settings still come from `~/.gemini/`
- Legacy `GMN_*` environment variables still work
- Legacy `~/.gmn/.env` is still loaded as a fallback

## Installation

Prerequisites:

- Go 1.22+
- A Gemini API key from [Google AI Studio](https://aistudio.google.com/apikey) or Google Cloud

Install from source after you create your GitHub repo:

```bash
go install github.com/PinkyFrog0o0/foxray@latest
```

This fork is now configured for the GitHub owner `PinkyFrog0o0`.

## Quick Start

```bash
# Simple prompt with API key
foxray "Explain quantum computing" -k YOUR_API_KEY

# Or set the env var once
export FOXRAY_API_KEY=YOUR_API_KEY
foxray "Explain quantum computing"

# Legacy env vars still work
export GMN_API_KEY=YOUR_API_KEY
foxray "Explain quantum computing"

# With file context
foxray "Review this code" -f main.go

# Pipe input
cat error.log | foxray "What's wrong?"

# JSON output
foxray "List 3 colors" -o json

# Use a different model
foxray "Write a poem" -m gemini-2.5-pro

# Vertex AI Express
foxray "Hello" --backend vertex -k YOUR_API_KEY
```

## Authentication

FoxRay supports two authentication modes.

API key mode:

```bash
foxray "Hello" -k YOUR_API_KEY
export FOXRAY_API_KEY=YOUR_API_KEY
foxray "Hello"
```

OAuth fallback:

If no API key is provided, FoxRay falls back to the existing Gemini CLI OAuth credentials stored in `~/.gemini/`.

```bash
npm install -g @google/gemini-cli
gemini
foxray "Hello"
```

## Environment Variables

FoxRay prefers the new `FOXRAY_*` environment variables and falls back to the legacy `GMN_*` names.

| Primary | Legacy fallback | Flag | Description | Default |
|---------|-----------------|------|-------------|---------|
| `FOXRAY_API_KEY` | `GMN_API_KEY` | `-k, --api-key` | Gemini / Vertex AI API key | — |
| `FOXRAY_BACKEND` | `GMN_BACKEND` | `--backend` | API backend (`gemini` or `vertex`) | `gemini` |
| `FOXRAY_MODEL` | `GMN_MODEL` | `-m, --model` | Model name | `gemini-2.5-flash` |
| `FOXRAY_API_URL` | `GMN_API_URL` | `--api-url` | Custom API base URL | auto |
| `FOXRAY_LOCATION` | `GMN_LOCATION` | `--location` | Vertex AI region | — |

## .env Loading

FoxRay loads `.env` files in this order:

1. `~/.foxray/.env`
2. `~/.gmn/.env`
3. `./.env`
4. OS environment variables override all file-based values

Example:

```dotenv
FOXRAY_API_KEY=AIza...
FOXRAY_MODEL=gemini-2.5-pro
FOXRAY_BACKEND=gemini
```

## MCP Support

FoxRay supports [Model Context Protocol](https://modelcontextprotocol.io/) servers through the Gemini CLI settings file at `~/.gemini/settings.json`.

```json
{
  "mcpServers": {
    "my-server": {
      "command": "/path/to/mcp-server"
    }
  }
}
```

```bash
foxray mcp list
foxray mcp call my-server tool-name arg=value
```

## Build

```bash
git clone https://github.com/PinkyFrog0o0/foxray.git
cd FoxRay
make build
make cross-compile
```

## Credits

FoxRay is a derivative work based on:

- [gmn-api](https://github.com/hirsaeki/gmn-api)
- [gmn](https://github.com/tomohiro-owada/gmn)
- [Google Gemini CLI](https://github.com/google-gemini/gemini-cli)
