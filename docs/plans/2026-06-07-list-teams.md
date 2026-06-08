# List Teams Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `teams-cli list teams` command that prints the authenticated user's teams with extra metadata fields.

**Architecture:** Extend argument parsing with a small command mode, then route execution through a dedicated non-TUI path that reuses existing Teams authentication and conversation fetch logic. Keep output generation in a small formatter so it can be tested without live API calls.

**Tech Stack:** Go, `teams-api`, standard library `text/tabwriter`, existing repo tests with `go test`

---

### Task 1: Parse the new subcommand

**Files:**
- Modify: `options.go`
- Test: `options_test.go`

**Step 1: Write the failing test**

Add a test that parses `[]string{"list", "teams"}` and asserts the new command mode is selected without enabling doctor mode.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestParseAppOptionsListTeams`
Expected: FAIL because the parser rejects the `list` argument or the new mode does not exist.

**Step 3: Write minimal implementation**

Add a command mode field to `AppOptions` and update `parseAppOptions` to accept `list teams`.

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestParseAppOptionsListTeams`
Expected: PASS

**Step 5: Commit**

```bash
git add options.go options_test.go
git commit -m "feat: parse list teams command"
```

### Task 2: Format team rows for stdout

**Files:**
- Create: `list_teams.go`
- Test: `state_teams_test.go`

**Step 1: Write the failing test**

Add a formatter test that passes a small list of teams and expects a header row plus values for name, ID, favorite, followed, archived, deleted, and channel count.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestFormatTeamsTable`
Expected: FAIL because the formatter does not exist.

**Step 3: Write minimal implementation**

Create a formatter that writes a stable table using `text/tabwriter`.

**Step 4: Run test to verify it passes**

Run: `go test ./... -run TestFormatTeamsTable`
Expected: PASS

**Step 5: Commit**

```bash
git add list_teams.go state_teams_test.go
git commit -m "feat: format team listing output"
```

### Task 3: Wire the command into program execution

**Files:**
- Modify: `main.go`
- Modify: `state_teams.go`
- Modify: `options.go`
- Test: `options_test.go`

**Step 1: Write the failing test**

Add a test that the parser still rejects incomplete `list` input such as `[]string{"list"}` with a helpful error.

**Step 2: Run test to verify it fails**

Run: `go test ./... -run TestParseAppOptionsListRequiresTarget`
Expected: FAIL because the parser does not yet distinguish incomplete `list` usage cleanly.

**Step 3: Write minimal implementation**

Add a command runner that initializes the Teams client, fetches conversations through shared state logic, sorts teams consistently, and prints the formatted table to stdout.

**Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestParseAppOptionsListTeams|TestParseAppOptionsListRequiresTarget|TestFormatTeamsTable'`
Expected: PASS

**Step 5: Commit**

```bash
git add main.go options.go state_teams.go options_test.go list_teams.go
git commit -m "feat: add list teams command"
```

### Task 4: Verify and install

**Files:**
- None

**Step 1: Run targeted tests**

Run: `go test ./...`
Expected: PASS

**Step 2: Build the binary**

Run: `go build -o ./dist/teams-cli ./`
Expected: binary created successfully

**Step 3: Install for local testing**

Run: `install -m 0755 ./dist/teams-cli ~/.local/bin/teams-cli`
Expected: updated executable in `~/.local/bin`

**Step 4: Smoke-test the new command**

Run: `~/.local/bin/teams-cli list teams`
Expected: a table of teams printed to stdout
