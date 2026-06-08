# List Teams Design

## Goal

Add a minimal non-interactive command that prints the Teams the authenticated user belongs to, so the tool can be used for quick inspection without opening the TUI.

## Scope

- Add `teams-cli list teams` as a new subcommand.
- Reuse the existing token and Teams client setup.
- Print one row per team with extra fields: display name, team ID, favorite, followed, archived, deleted, and channel count.
- Keep team ordering consistent with the TUI by reusing the existing team sorting logic.

## Approach

The new subcommand will share the same data source as the TUI. After parsing arguments, the command path will initialize the Teams client, fetch conversations, sort the team list with the existing `sortTeams`, and print a tabular stdout view.

This keeps the first implementation small and avoids introducing new output formats or duplicate fetch code. JSON, filtering, and richer formatting can be added later if the command proves useful.

## Error Handling

- Reuse existing token resolution behavior and `--token-dir`.
- Return non-zero on client initialization or conversation fetch failures.
- Print user-facing errors to stderr, matching the existing CLI style.

## Testing

- Add parser coverage for `list teams`.
- Add a formatter test that verifies headers and the expected extra fields.
- Keep the implementation testable without live network calls by isolating formatting and using the existing sorting helper.
