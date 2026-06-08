package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestParseAppOptionsDefaults(t *testing.T) {
	options, err := parseAppOptions(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.MessageLimit != defaultMessageLimit {
		t.Fatalf("expected default message limit %d, got %d", defaultMessageLimit, options.MessageLimit)
	}
	if options.LogLevel != logrus.InfoLevel {
		t.Fatalf("expected default log level info, got %s", options.LogLevel)
	}
	if !options.LiveRefresh {
		t.Fatal("expected live refresh to be enabled by default")
	}
	if options.RefreshMessagesInterval != defaultLiveMessageRefreshInterval {
		t.Fatalf("expected default message refresh interval %s, got %s", defaultLiveMessageRefreshInterval, options.RefreshMessagesInterval)
	}
	if options.RefreshConversationInterval != defaultLiveConversationRefreshInterval {
		t.Fatalf("expected default tree refresh interval %s, got %s", defaultLiveConversationRefreshInterval, options.RefreshConversationInterval)
	}
}

func TestParseAppOptionsExtendedFlags(t *testing.T) {
	options, err := parseAppOptions([]string{
		"--msg", "25",
		"--debug",
		"--token-dir", "/tmp/tokens",
		"--refresh-messages", "10",
		"--refresh-tree=45s",
		"--no-live",
		"doctor",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.MessageLimit != 25 {
		t.Fatalf("expected message limit 25, got %d", options.MessageLimit)
	}
	if options.LogLevel != logrus.DebugLevel {
		t.Fatalf("expected debug log level, got %s", options.LogLevel)
	}
	if options.TokenDir != "/tmp/tokens" {
		t.Fatalf("expected token dir /tmp/tokens, got %q", options.TokenDir)
	}
	if options.RefreshMessagesInterval != 10*time.Second {
		t.Fatalf("expected message refresh 10s, got %s", options.RefreshMessagesInterval)
	}
	if options.RefreshConversationInterval != 45*time.Second {
		t.Fatalf("expected tree refresh 45s, got %s", options.RefreshConversationInterval)
	}
	if options.LiveRefresh {
		t.Fatal("expected live refresh to be disabled")
	}
	if !options.DoctorMode {
		t.Fatal("expected doctor mode to be enabled")
	}
}

func TestParseAppOptionsLegacyMessageLimit(t *testing.T) {
	options, err := parseAppOptions([]string{"msg=25"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.MessageLimit != 25 {
		t.Fatalf("expected message limit 25, got %d", options.MessageLimit)
	}
}

func TestParseAppOptionsHelpAndVersion(t *testing.T) {
	options, err := parseAppOptions([]string{"--help", "--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !options.ShowHelp {
		t.Fatal("expected help mode")
	}
	if !options.ShowVersion {
		t.Fatal("expected version mode")
	}
}

func TestParseAppOptionsDebugFlagCanBeOverridden(t *testing.T) {
	options, err := parseAppOptions([]string{"--debug", "--log-level", "error"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.LogLevel != logrus.ErrorLevel {
		t.Fatalf("expected error log level, got %s", options.LogLevel)
	}
}

func TestParseAppOptionsSupportsEqualsForms(t *testing.T) {
	options, err := parseAppOptions([]string{
		"--msg=12",
		"--log-level=warn",
		"--token-dir=/tmp/teams-cli-tokens",
		"--refresh-messages=12s",
		"--refresh-tree=30",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.MessageLimit != 12 {
		t.Fatalf("expected message limit 12, got %d", options.MessageLimit)
	}
	if options.LogLevel != logrus.WarnLevel {
		t.Fatalf("expected warn level, got %s", options.LogLevel)
	}
	if options.TokenDir != "/tmp/teams-cli-tokens" {
		t.Fatalf("expected token dir to be parsed, got %q", options.TokenDir)
	}
	if options.RefreshMessagesInterval != 12*time.Second {
		t.Fatalf("expected 12s message refresh interval, got %s", options.RefreshMessagesInterval)
	}
	if options.RefreshConversationInterval != 30*time.Second {
		t.Fatalf("expected 30s tree refresh interval, got %s", options.RefreshConversationInterval)
	}
}

func TestParseAppOptionsListTeams(t *testing.T) {
	options, err := parseAppOptions([]string{"list", "teams"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.DoctorMode {
		t.Fatal("expected doctor mode to remain disabled")
	}
	if options.CommandMode != commandModeListTeams {
		t.Fatalf("expected command mode %q, got %q", commandModeListTeams, options.CommandMode)
	}
}

func TestParseAppOptionsReadChannelJSON(t *testing.T) {
	options, err := parseAppOptions([]string{"read", "channel", "19:channel", "--limit", "50", "--since", "24h", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeReadChannel {
		t.Fatalf("expected command mode %q, got %q", commandModeReadChannel, options.CommandMode)
	}
	if options.Command.TargetID != "19:channel" {
		t.Fatalf("expected target id to be parsed, got %q", options.Command.TargetID)
	}
	if options.Command.ResultLimit != 50 {
		t.Fatalf("expected result limit 50, got %d", options.Command.ResultLimit)
	}
	if options.Command.Since != "24h" {
		t.Fatalf("expected since to be parsed, got %q", options.Command.Since)
	}
	if !options.Command.OutputJSON {
		t.Fatal("expected json output mode")
	}
}

func TestParseAppOptionsSearchMessages(t *testing.T) {
	options, err := parseAppOptions([]string{"search", "messages", "--query", "incident", "--team", "team-a", "--channel", "channel-a", "--limit", "20"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeSearchMessages {
		t.Fatalf("expected command mode %q, got %q", commandModeSearchMessages, options.CommandMode)
	}
	if options.Command.Query != "incident" {
		t.Fatalf("expected query to be parsed, got %q", options.Command.Query)
	}
	if len(options.Command.TeamIDs) != 1 || options.Command.TeamIDs[0] != "team-a" {
		t.Fatalf("expected team selector to be parsed, got %#v", options.Command.TeamIDs)
	}
	if len(options.Command.ChannelIDs) != 1 || options.Command.ChannelIDs[0] != "channel-a" {
		t.Fatalf("expected channel selector to be parsed, got %#v", options.Command.ChannelIDs)
	}
	if options.Command.ResultLimit != 20 {
		t.Fatalf("expected result limit 20, got %d", options.Command.ResultLimit)
	}
}

func TestParseAppOptionsWatchMentions(t *testing.T) {
	options, err := parseAppOptions([]string{"watch", "mentions", "--profile", "focus", "--poll", "30s", "--state-file", "/tmp/watch.json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeWatchMentions {
		t.Fatalf("expected command mode %q, got %q", commandModeWatchMentions, options.CommandMode)
	}
	if options.Command.Profile != "focus" {
		t.Fatalf("expected profile to be parsed, got %q", options.Command.Profile)
	}
	if options.Command.PollInterval != 30*time.Second {
		t.Fatalf("expected poll interval 30s, got %s", options.Command.PollInterval)
	}
	if options.Command.StateFile != "/tmp/watch.json" {
		t.Fatalf("expected state file to be parsed, got %q", options.Command.StateFile)
	}
}

func TestParseAppOptionsListMentionsIncludeNameMentions(t *testing.T) {
	options, err := parseAppOptions([]string{"list", "mentions", "--include-name-mentions"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeListMentions {
		t.Fatalf("expected command mode %q, got %q", commandModeListMentions, options.CommandMode)
	}
	if !options.Command.IncludeNameMentions {
		t.Fatal("expected include-name-mentions flag to be enabled")
	}
}

func TestParseAppOptionsListCalendars(t *testing.T) {
	options, err := parseAppOptions([]string{"list", "calendars", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeListCalendars {
		t.Fatalf("expected command mode %q, got %q", commandModeListCalendars, options.CommandMode)
	}
	if !options.Command.OutputJSON {
		t.Fatal("expected json output mode")
	}
}

func TestParseAppOptionsListEventsDefaultsRange(t *testing.T) {
	options, err := parseAppOptions([]string{"list", "events", "--calendar", "cal-1", "--limit", "10"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeListEvents {
		t.Fatalf("expected command mode %q, got %q", commandModeListEvents, options.CommandMode)
	}
	if options.Command.CalendarID != "cal-1" {
		t.Fatalf("expected calendar id cal-1, got %q", options.Command.CalendarID)
	}
	if options.Command.ResultLimit != 10 {
		t.Fatalf("expected result limit 10, got %d", options.Command.ResultLimit)
	}
	if strings.TrimSpace(options.Command.Start) == "" || strings.TrimSpace(options.Command.End) == "" {
		t.Fatal("expected default event range to be populated")
	}
}

func TestParseAppOptionsCreateEvent(t *testing.T) {
	options, err := parseAppOptions([]string{
		"create", "event",
		"--calendar", "cal-1",
		"--subject", "Design review",
		"--start", "2026-06-08T09:00:00Z",
		"--end", "2026-06-08T10:00:00Z",
		"--timezone", "Europe/Zurich",
		"--location", "Room 1",
		"--body", "Agenda",
		"--all-day",
		"--json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeCreateEvent {
		t.Fatalf("expected command mode %q, got %q", commandModeCreateEvent, options.CommandMode)
	}
	if options.Command.Subject != "Design review" {
		t.Fatalf("expected subject to be parsed, got %q", options.Command.Subject)
	}
	if options.Command.Timezone != "Europe/Zurich" {
		t.Fatalf("expected timezone to be parsed, got %q", options.Command.Timezone)
	}
	if !options.Command.AllDay {
		t.Fatal("expected all-day flag to be enabled")
	}
	if !options.Command.OutputJSON {
		t.Fatal("expected json output mode")
	}
}

func TestParseAppOptionsUpdateEvent(t *testing.T) {
	options, err := parseAppOptions([]string{"update", "event", "evt-1", "--subject", "Updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeUpdateEvent {
		t.Fatalf("expected command mode %q, got %q", commandModeUpdateEvent, options.CommandMode)
	}
	if options.Command.EventID != "evt-1" {
		t.Fatalf("expected event id evt-1, got %q", options.Command.EventID)
	}
	if options.Command.Subject != "Updated" {
		t.Fatalf("expected subject Updated, got %q", options.Command.Subject)
	}
}

func TestParseAppOptionsDeleteEvent(t *testing.T) {
	options, err := parseAppOptions([]string{"delete", "event", "evt-2", "--calendar", "cal-2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if options.CommandMode != commandModeDeleteEvent {
		t.Fatalf("expected command mode %q, got %q", commandModeDeleteEvent, options.CommandMode)
	}
	if options.Command.EventID != "evt-2" {
		t.Fatalf("expected event id evt-2, got %q", options.Command.EventID)
	}
	if options.Command.CalendarID != "cal-2" {
		t.Fatalf("expected calendar id cal-2, got %q", options.Command.CalendarID)
	}
}

func TestParseAppOptionsListRequiresTarget(t *testing.T) {
	_, err := parseAppOptions([]string{"list"})
	if err == nil {
		t.Fatal("expected an error for incomplete list command")
	}
	if !strings.Contains(err.Error(), "missing list target") {
		t.Fatalf("expected missing list target error, got %v", err)
	}
}

func TestParseAppOptionsRejectsCreateEventWithoutRequiredFields(t *testing.T) {
	_, err := parseAppOptions([]string{"create", "event", "--subject", "Design review"})
	if err == nil {
		t.Fatal("expected an error for incomplete create event command")
	}
	if !strings.Contains(err.Error(), "create event requires") {
		t.Fatalf("expected create event validation error, got %v", err)
	}
}

func TestParseAppOptionsRejectsUpdateEventWithoutChanges(t *testing.T) {
	_, err := parseAppOptions([]string{"update", "event", "evt-1"})
	if err == nil {
		t.Fatal("expected an error for update without changes")
	}
	if !strings.Contains(err.Error(), "update event requires at least one field") {
		t.Fatalf("expected update validation error, got %v", err)
	}
}

func TestParseFlexibleDurationNumericSeconds(t *testing.T) {
	got, err := parseFlexibleDuration("30")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 30*time.Second {
		t.Fatalf("expected 30s, got %s", got)
	}
}

func TestParseFlexibleDurationDays(t *testing.T) {
	got, err := parseFlexibleDuration("7d")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != 7*24*time.Hour {
		t.Fatalf("expected 7 days, got %s", got)
	}
}

func TestParseAppOptionsRejectsInvalidMessageLimit(t *testing.T) {
	_, err := parseAppOptions([]string{"msg=0"})
	if err == nil {
		t.Fatal("expected an error for msg=0")
	}
}

func TestParseAppOptionsRejectsInvalidRefreshInterval(t *testing.T) {
	_, err := parseAppOptions([]string{"--refresh-messages", "0"})
	if err == nil {
		t.Fatal("expected an error for zero refresh interval")
	}
}

func TestParseAppOptionsRejectsSearchWithoutQuery(t *testing.T) {
	_, err := parseAppOptions([]string{"search", "messages"})
	if err == nil {
		t.Fatal("expected an error for missing search query")
	}
	if !strings.Contains(err.Error(), "query") {
		t.Fatalf("expected query error, got %v", err)
	}
}

func TestParseAppOptionsRejectsMissingFlagValues(t *testing.T) {
	testCases := []string{
		"--msg",
		"--log-level",
		"--token-dir",
		"--refresh-messages",
		"--refresh-tree",
	}

	for _, arg := range testCases {
		if _, err := parseAppOptions([]string{arg}); err == nil {
			t.Fatalf("expected an error for %s without a value", arg)
		}
	}
}

func TestParseAppOptionsRejectsUnknownArgument(t *testing.T) {
	_, err := parseAppOptions([]string{"foo=bar"})
	if err == nil {
		t.Fatal("expected an error for an unknown argument")
	}
}

func TestValidateRuntimeTokensRejectsExpiredRequiredTokens(t *testing.T) {
	dir := t.TempDir()
	expired := testJWT(t, map[string]any{
		"aud": "https://api.spaces.skype.com",
		"exp": time.Now().Add(-time.Hour).Unix(),
	})
	if err := os.WriteFile(filepath.Join(dir, "token-skype.jwt"), []byte(expired), 0o600); err != nil {
		t.Fatalf("unable to write skype token: %v", err)
	}
	chatsvcagg := testJWT(t, map[string]any{
		"aud": "https://chatsvcagg.teams.microsoft.com",
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	if err := os.WriteFile(filepath.Join(dir, "token-chatsvcagg.jwt"), []byte(chatsvcagg), 0o600); err != nil {
		t.Fatalf("unable to write chatsvcagg token: %v", err)
	}

	err := validateRuntimeTokens(dir)
	if err == nil {
		t.Fatal("expected expired runtime tokens to be rejected")
	}
	if !strings.Contains(err.Error(), "expired") {
		t.Fatalf("expected expiry guidance, got %v", err)
	}
}

func TestUsageTextIncludesDiagnosticsAndRefreshFlags(t *testing.T) {
	text := usageText("teams-cli")

	for _, needle := range []string{"--doctor", "--no-live", "--refresh-messages", "--refresh-tree", "list teams", "read channel", "search messages", "watch mentions"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected usage text to mention %s", needle)
		}
	}
}
