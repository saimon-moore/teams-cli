package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type CLIProfile struct {
	TeamIDs        []string `json:"team_ids"`
	ChannelIDs     []string `json:"channel_ids"`
	ChatIDs        []string `json:"chat_ids"`
	MentionAliases []string `json:"mention_aliases"`
}

type CLIConfig struct {
	Profiles map[string]CLIProfile `json:"profiles"`
}

type CommandSelectors struct {
	TeamIDs        []string
	ChannelIDs     []string
	ChatIDs        []string
	MentionAliases []string
}

func defaultCLIConfigPath() (string, error) {
	tokenDir, err := defaultTokenDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(tokenDir, "teams-cli.json"), nil
}

func loadCLIConfig(path string) (CLIConfig, error) {
	if strings.TrimSpace(path) == "" {
		return CLIConfig{Profiles: map[string]CLIProfile{}}, nil
	}

	body, err := os.ReadFile(path)
	if err != nil {
		return CLIConfig{}, fmt.Errorf("unable to read config %s: %v", path, err)
	}

	config := CLIConfig{Profiles: map[string]CLIProfile{}}
	if err := json.Unmarshal(body, &config); err != nil {
		return CLIConfig{}, fmt.Errorf("unable to decode config %s: %v", path, err)
	}
	if config.Profiles == nil {
		config.Profiles = map[string]CLIProfile{}
	}

	return config, nil
}

func loadCLIConfigIfPresent(path string) (CLIConfig, error) {
	if strings.TrimSpace(path) == "" {
		return CLIConfig{Profiles: map[string]CLIProfile{}}, nil
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return CLIConfig{Profiles: map[string]CLIProfile{}}, nil
		}
		return CLIConfig{}, err
	}

	return loadCLIConfig(path)
}

func resolveCommandSelectors(command CommandOptions, config CLIConfig) (CommandSelectors, error) {
	selectors := CommandSelectors{}
	profile := CLIProfile{}
	if command.Profile != "" {
		var ok bool
		profile, ok = config.Profiles[command.Profile]
		if !ok {
			return selectors, fmt.Errorf("unknown profile %q", command.Profile)
		}
	}

	if len(command.TeamIDs) > 0 {
		selectors.TeamIDs = appendUniqueNormalized(nil, command.TeamIDs...)
	} else {
		selectors.TeamIDs = appendUniqueNormalized(nil, profile.TeamIDs...)
	}
	if len(command.ChannelIDs) > 0 {
		selectors.ChannelIDs = appendUniqueNormalized(nil, command.ChannelIDs...)
	} else {
		selectors.ChannelIDs = appendUniqueNormalized(nil, profile.ChannelIDs...)
	}
	if len(command.ChatIDs) > 0 {
		selectors.ChatIDs = appendUniqueNormalized(nil, command.ChatIDs...)
	} else {
		selectors.ChatIDs = appendUniqueNormalized(nil, profile.ChatIDs...)
	}
	selectors.MentionAliases = appendUniqueNormalized(nil, profile.MentionAliases...)

	return selectors, nil
}

func appendUniqueNormalized(base []string, values ...string) []string {
	seen := map[string]struct{}{}
	for _, existing := range base {
		seen[strings.ToLower(existing)] = struct{}{}
	}

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		base = append(base, value)
	}

	sort.Strings(base)
	return base
}
