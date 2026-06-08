package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCLIConfigAndResolveSelectors(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "teams-cli.json")
	configBody := `{
  "profiles": {
    "focus": {
      "team_ids": ["team-from-profile"],
      "channel_ids": ["channel-from-profile"],
      "chat_ids": ["chat-from-profile"],
      "mention_aliases": ["Saimon", "@saimon"]
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("unable to write config: %v", err)
	}

	config, err := loadCLIConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	selectors, err := resolveCommandSelectors(CommandOptions{
		Profile:    "focus",
		ChannelIDs: []string{"channel-explicit"},
	}, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(selectors.TeamIDs) != 1 || selectors.TeamIDs[0] != "team-from-profile" {
		t.Fatalf("expected profile team ids, got %#v", selectors.TeamIDs)
	}
	if len(selectors.ChannelIDs) != 1 || selectors.ChannelIDs[0] != "channel-explicit" {
		t.Fatalf("expected explicit channel ids to override profile, got %#v", selectors.ChannelIDs)
	}
	if len(selectors.ChatIDs) != 1 || selectors.ChatIDs[0] != "chat-from-profile" {
		t.Fatalf("expected profile chat ids, got %#v", selectors.ChatIDs)
	}
	if len(selectors.MentionAliases) != 2 {
		t.Fatalf("expected mention aliases to be loaded, got %#v", selectors.MentionAliases)
	}
	joined := selectors.MentionAliases[0] + " " + selectors.MentionAliases[1]
	if joined != "@saimon Saimon" && joined != "Saimon @saimon" {
		t.Fatalf("expected mention aliases to be preserved, got %#v", selectors.MentionAliases)
	}
}

func TestResolveCommandSelectorsRejectsUnknownProfile(t *testing.T) {
	_, err := resolveCommandSelectors(CommandOptions{Profile: "missing"}, CLIConfig{
		Profiles: map[string]CLIProfile{},
	})
	if err == nil {
		t.Fatal("expected an error for an unknown profile")
	}
}
