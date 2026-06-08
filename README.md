# teams-cli

A Command Line Interface (or TUI) to interact with Microsoft Teams that uses the [teams-api](https://github.com/fossteams/teams-api) Go package.

## Status

Upstream `fossteams/teams-cli` has been archived and is read-only. This fork is the active maintenance branch for the codebase, and new work should target this repository.

This project is still WIP and will be updated with more features. The goal is to have a CLI / TUI replacement for the Microsoft Teams desktop client. Today the client is primarily read-only (browsing conversations and reading recent messages).

Release automation is driven from the maintained release branches in GitHub Actions.

## Documentation

**We have moved our comprehensive documentation to the Wiki.** 

Please browse the [GitHub Wiki](https://github.com/vaishnavucv/teams-cli/wiki) for detailed guides:
- [Installation Guide](https://github.com/vaishnavucv/teams-cli/wiki/Installation)
- [Usage & Keyboard Navigation](https://github.com/vaishnavucv/teams-cli/wiki/Usage)
- [Features List](https://github.com/vaishnavucv/teams-cli/wiki/Features)
- [Development, CI/CD, & Governance](https://github.com/vaishnavucv/teams-cli/wiki/CI-CD)

## Quick Start

### Requirements
- [Golang](https://golang.org/) 1.26.1 or newer
- Valid Teams JWT files generated with [teams-token](https://github.com/fossteams/teams-token)
- A terminal with cursor-addressing support (e.g. Terminal.app, iTerm2)

### Basic Usage

Run the app locally once you have obtained your tokens:
```bash
go run ./
```

To limit each conversation view to the most recent `N` messages:
```bash
go run ./ msg=20
```

Run diagnostics before using either the TUI or the read-only CLI commands:
```bash
go run ./ doctor
```

*For more runtime flags and usage examples, see the [Usage Wiki](wiki/Usage.md).*

### Read-Only CLI Commands

The project now also exposes a set of non-interactive commands for scripting, export, and LLM-assisted analysis. These commands reuse the same Teams authentication as the TUI and default to text output, with `--json` available for machine-readable output.

Calendar commands are Graph-native rather than Teams-native. They use the
optional `token-graph.jwt` / `MS_TEAMS_GRAPH_TOKEN` auth path and do not require
the Skype or ChatSvcAgg bootstrap tokens just to inspect or mutate calendar data.

#### Discovery

List the Teams you belong to:
```bash
go run ./ list teams
```

List channels, optionally filtered to one or more Teams:
```bash
go run ./ list channels
go run ./ list channels --team 19:team-id --json
```

List available calendars from Microsoft Graph:
```bash
go run ./ list calendars
go run ./ list calendars --json
```

List upcoming events from the default calendar. When `--start` and `--end` are
omitted, the command defaults to the next 7 days in UTC:
```bash
go run ./ list events
go run ./ list events --calendar cal-id --start 2026-06-08T00:00:00Z --end 2026-06-15T00:00:00Z --json
```

Create, update, or delete events using exact event IDs and optional explicit
calendar IDs:
```bash
go run ./ create event --subject "Design review" --start 2026-06-08T09:00:00Z --end 2026-06-08T10:00:00Z --timezone Europe/Zurich --location "Room 1"
go run ./ update event evt-id --calendar cal-id --subject "Updated title"
go run ./ delete event evt-id --calendar cal-id
```

List messages that explicitly `@mention` you:
```bash
go run ./ list mentions --since 7d
go run ./ list mentions --channel 19:channel-id --limit 20 --json
```

Text output prints the matching message body under each hit. JSON output includes the full body in `message_text`:
```bash
go run ./ list mentions --since 7d --json | jq '.[0].message_text'
```

Include name or alias matching explicitly when needed:
```bash
go run ./ list mentions --since 7d --include-name-mentions
go run ./ list mentions --profile focus --include-name-mentions --limit 20 --json
```

#### Reading Conversations

Read recent messages from a channel:
```bash
go run ./ read channel 19:channel-id --limit 50
go run ./ read channel 19:channel-id --since 24h --json
```

Read recent messages from a chat:
```bash
go run ./ read chat 19:chat-id --limit 50
go run ./ read chat 19:chat-id --since 7d --json
```

Both `read` commands preserve thread relationships when Teams exposes reply metadata.

#### Export

Export a conversation for downstream tooling or LLM ingestion:
```bash
go run ./ export channel 19:channel-id --limit 200 --format json --output /tmp/channel.json
go run ./ export chat 19:chat-id --since 30d --format md --output /tmp/chat.md
```

#### Search and Watch

Search message history client-side within the selected conversations:
```bash
go run ./ search messages --query incident --since 7d
go run ./ search messages --query deploy --channel 19:channel-id --json
```

Like mention results, search results include the full matched message body in text mode and in the JSON `message_text` field:
```bash
go run ./ search messages --query incident --since 7d --json | jq '.[0].message_text'
```

Limit the search scope aggressively on large accounts by repeating `--team` or `--channel`:
```bash
go run ./ search messages --query ucl --team 19:team-a --team 19:team-b --since 7d
go run ./ search messages --query ucl --channel 19:channel-a --channel 19:channel-b --limit 20 --json
go run ./ search messages --profile focus --query outage --since 7d --json
```

Watch for new mention hits using a persisted local checkpoint:
```bash
go run ./ watch mentions --since 1d --poll 30s
go run ./ watch mentions --channel 19:channel-id --state-file /tmp/teams-cli-watch.json --json
```

Opt into alias/name matching for watch mode only when you want it:
```bash
go run ./ watch mentions --profile focus --since 1d --include-name-mentions
```

#### Profiles and Selectors

For repeated analysis workflows, you can store named profiles in `~/.config/fossteams/teams-cli.json` and select them with `--profile`.

Example config:
```json
{
  "profiles": {
    "focus": {
      "team_ids": ["19:team-id"],
      "channel_ids": ["19:channel-id"],
      "chat_ids": ["19:chat-id"],
      "mention_aliases": ["Simon", "@simon"]
    }
  }
}
```

Example usage:
```bash
go run ./ list mentions --profile focus --since 7d
go run ./ search messages --profile focus --query outage --json
```

Selectors passed on the command line override the same selector type from the profile.
This makes `--profile` the fastest way to keep broad scans focused on a small set of teams, channels, or chats.

## Lite Overview

### Core Features
- Browse your Teams, Channels, and direct/group Chats from a TUI.
- Automatically load and read recent messages in selected conversations.
- Query Teams, channels, chats, and mentions from non-interactive CLI commands.
- Export channel and chat history as JSON or Markdown.
- Search recent message history and watch for new mention hits.
- Background refresh for both message lists and conversation trees.
- Navigate the UI entirely with keyboard shortcuts.
- Clean shutdown on `q` or `Ctrl+C`.

### Keyboard Navigation
- **Arrow Keys**: Move up/down lists, expand (`Right`) or collapse (`Left`) trees.
- **Enter**: Open the selected channel or chat.
- **Tab**: Switch focus between the conversations tree and messages pane.
- **Esc**: Go back one level or return to the conversations tree.
- **?**: Show/hide the full keyboard help inside the app.
- **q**: Quit the app.

### Diagnostics
If you have issues logging in or starting the app, run the built-in doctor to test token validity and network reachability:
```bash
go run ./ doctor
```

If command mode fails unexpectedly, `doctor` will also tell you whether the required runtime tokens are missing or expired.

## Support

- Found an issue or have a feature request? Please use our **built-in issue templates**.
- Found a security vulnerability? Please review our **[SECURITY.md](./SECURITY.md)** guidelines privately before opening a public issue.
- View our **[CHANGELOG.md](./CHANGELOG.md)** for detailed release notes and updates.
- This fork maintains the `github.com/fossteams/teams-cli` module path for compatibility, but the releases published here are the officially supported install path.
