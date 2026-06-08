package main

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	teams_api "github.com/saimon-moore/teams-api"
	"github.com/sirupsen/logrus"
)

type commandEnvironment struct {
	service       commandService
	teamsState    TeamsState
	config        CLIConfig
	selectors     CommandSelectors
	conversations []conversationDescriptor
}

type mentionHit struct {
	Timestamp            time.Time `json:"timestamp"`
	Sender               string    `json:"sender"`
	ConversationType     string    `json:"conversation_type"`
	ConversationTitle    string    `json:"conversation_title"`
	TeamID               string    `json:"team_id,omitempty"`
	ChannelID            string    `json:"channel_id,omitempty"`
	ChatID               string    `json:"chat_id,omitempty"`
	MessageID            string    `json:"message_id"`
	ThreadRootSequenceID int64     `json:"thread_root_sequence_id"`
	MatchType            string    `json:"match_type"`
	MessageText          string    `json:"message_text"`
	Preview              string    `json:"preview"`
}

type searchHit struct {
	Timestamp         time.Time `json:"timestamp"`
	ConversationType  string    `json:"conversation_type"`
	ConversationTitle string    `json:"conversation_title"`
	MessageID         string    `json:"message_id"`
	Sender            string    `json:"sender"`
	MessageText       string    `json:"message_text"`
	Preview           string    `json:"preview"`
}

type watchStateFile struct {
	Checkpoints map[string]watchCheckpoint `json:"checkpoints"`
}

type watchCheckpoint struct {
	LastTimestamp string `json:"last_timestamp"`
	LastMessageID string `json:"last_message_id"`
}

func runCommand(ctx context.Context, out io.Writer, options AppOptions, logger *logrus.Logger) error {
	if commandUsesGraphBootstrap(options.CommandMode) {
		return runCalendarCommand(ctx, out, options, logger)
	}

	env, err := prepareCommandEnvironment(options, logger)
	if err != nil {
		return err
	}

	switch options.CommandMode {
	case commandModeListTeams:
		return formatTeamsTable(out, env.teamsState.conversations.Teams)
	case commandModeListChannels:
		return runListChannels(out, env, options)
	case commandModeReadChannel, commandModeReadChat:
		return runReadConversation(ctx, out, env, options)
	case commandModeExportChannel, commandModeExportChat:
		return runExportConversation(ctx, out, env, options)
	case commandModeListMentions:
		return runListMentions(ctx, out, env, options)
	case commandModeSearchMessages:
		return runSearchMessages(ctx, out, env, options)
	case commandModeWatchMentions:
		return runWatchMentions(ctx, out, env, options)
	default:
		return fmt.Errorf("unsupported command mode %q", options.CommandMode)
	}
}

func prepareCommandEnvironment(options AppOptions, logger *logrus.Logger) (*commandEnvironment, error) {
	client, err := teams_api.New()
	if err != nil {
		return nil, fmt.Errorf("unable to initialize teams client: %v", err)
	}

	state := TeamsState{teamsClient: client}
	if err := state.init(client); err != nil {
		return nil, err
	}

	configPath := options.Command.ConfigPath
	if strings.TrimSpace(configPath) == "" {
		configPath, err = defaultCLIConfigPath()
		if err != nil {
			return nil, err
		}
	}

	var config CLIConfig
	if options.Command.Profile != "" {
		config, err = loadCLIConfig(configPath)
	} else {
		config, err = loadCLIConfigIfPresent(configPath)
	}
	if err != nil {
		return nil, err
	}

	selectors, err := resolveCommandSelectors(options.Command, config)
	if err != nil {
		return nil, err
	}

	return &commandEnvironment{
		service: commandService{
			teamsClient: client,
			logger:      logger,
		},
		teamsState:    state,
		config:        config,
		selectors:     selectors,
		conversations: buildConversationDescriptors(&state),
	}, nil
}

func runListChannels(out io.Writer, env *commandEnvironment, options AppOptions) error {
	channels := filterConversationDescriptors(env.conversations, env.selectors, conversationTypeChannel)
	if options.Command.OutputJSON {
		return writeJSON(out, channels)
	}

	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "TEAM\tTEAM_ID\tCHANNEL\tCHANNEL_ID\tGENERAL\tFAVORITE\tFOLLOWED\tARCHIVED\tDELETED"); err != nil {
		return err
	}
	for _, descriptor := range channels {
		channel := env.teamsState.channelById[descriptor.ChannelID]
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			descriptor.TeamName,
			descriptor.TeamID,
			descriptor.ChannelName,
			descriptor.ChannelID,
			strconv.FormatBool(channel.IsGeneral),
			strconv.FormatBool(channel.IsFavorite),
			strconv.FormatBool(channel.IsFollowed),
			strconv.FormatBool(channel.IsArchived),
			strconv.FormatBool(channel.IsDeleted),
		); err != nil {
			return err
		}
	}

	return writer.Flush()
}

func runReadConversation(ctx context.Context, out io.Writer, env *commandEnvironment, options AppOptions) error {
	descriptor, err := resolveConversationTarget(env, options.CommandMode, options.Command.TargetID)
	if err != nil {
		return err
	}

	since, err := parseSinceValue(options.Command.Since)
	if err != nil {
		return err
	}
	rawMessages, err := env.service.fetchMessageHistory(ctx, descriptor.Target, messageFetchOptions{
		PageSize:    defaultMessageLimit,
		MaxMessages: options.Command.ResultLimit,
		Since:       since,
	})
	if err != nil {
		return err
	}
	normalized := normalizeConversationMessages(descriptor, rawMessages, env.teamsState.me, env.selectors.MentionAliases)

	if options.Command.OutputJSON {
		payload := map[string]any{
			"conversation": descriptor,
			"messages":     normalized,
		}
		return writeJSON(out, payload)
	}

	return renderThreadedText(out, descriptor, normalized)
}

func runExportConversation(ctx context.Context, out io.Writer, env *commandEnvironment, options AppOptions) error {
	buffer := &strings.Builder{}
	descriptor, err := resolveConversationTarget(env, options.CommandMode, options.Command.TargetID)
	if err != nil {
		return err
	}
	since, err := parseSinceValue(options.Command.Since)
	if err != nil {
		return err
	}
	rawMessages, err := env.service.fetchMessageHistory(ctx, descriptor.Target, messageFetchOptions{
		PageSize:    defaultMessageLimit,
		MaxMessages: options.Command.ResultLimit,
		Since:       since,
	})
	if err != nil {
		return err
	}
	normalized := normalizeConversationMessages(descriptor, rawMessages, env.teamsState.me, env.selectors.MentionAliases)

	switch options.Command.Format {
	case "json":
		if err := writeJSON(buffer, map[string]any{
			"conversation": descriptor,
			"messages":     normalized,
		}); err != nil {
			return err
		}
	case "md":
		if err := renderThreadedMarkdown(buffer, descriptor, normalized); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported export format %q", options.Command.Format)
	}

	return writeCommandOutput(out, options.Command.OutputPath, buffer.String())
}

func runListMentions(ctx context.Context, out io.Writer, env *commandEnvironment, options AppOptions) error {
	hits, err := collectMentionHits(ctx, env, options)
	if err != nil {
		return err
	}
	if options.Command.OutputJSON {
		return writeJSON(out, hits)
	}

	return renderMentionHitsText(out, hits)
}

func runSearchMessages(ctx context.Context, out io.Writer, env *commandEnvironment, options AppOptions) error {
	hits, err := collectSearchHits(ctx, env, options)
	if err != nil {
		return err
	}
	if options.Command.OutputJSON {
		return writeJSON(out, hits)
	}

	return renderSearchHitsText(out, hits)
}

func runWatchMentions(ctx context.Context, out io.Writer, env *commandEnvironment, options AppOptions) error {
	stateFile, err := resolveWatchStateFile(options, env.selectors)
	if err != nil {
		return err
	}

	pollEvery := options.Command.PollInterval
	if pollEvery <= 0 {
		pollEvery = 30 * time.Second
	}

	for {
		hits, checkpoint, err := collectWatchMentionHits(ctx, env, options, stateFile)
		if err != nil {
			return err
		}
		if len(hits) > 0 {
			if options.Command.OutputJSON {
				if err := writeJSON(out, hits); err != nil {
					return err
				}
			} else {
				if err := renderMentionHitsText(out, hits); err != nil {
					return err
				}
			}
			if err := persistWatchCheckpoint(stateFile, selectorStateKey(env.selectors), checkpoint); err != nil {
				return err
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollEvery):
		}
	}
}

func collectMentionHits(ctx context.Context, env *commandEnvironment, options AppOptions) ([]mentionHit, error) {
	since, err := parseSinceValue(options.Command.Since)
	if err != nil {
		return nil, err
	}

	descriptors := prioritizeConversationDescriptors(filterConversationDescriptors(env.conversations, env.selectors, ""), since)
	hits := make([]mentionHit, 0, options.Command.ResultLimit)
	failureCount := 0
	var lastErr error
	for _, descriptor := range descriptors {
		rawMessages, err := env.service.fetchMessageHistory(ctx, descriptor.Target, messageFetchOptions{
			PageSize: defaultMessageLimit,
			Since:    since,
		})
		if err != nil {
			if shouldSkipConversationScanError(descriptors, err) {
				failureCount++
				lastErr = err
				logSkippedConversationFetch(env, descriptor, err)
				continue
			}
			return nil, err
		}
		normalized := normalizeConversationMessages(descriptor, rawMessages, env.teamsState.me, env.selectors.MentionAliases)
		for _, message := range normalized {
			matchType := ""
			switch {
			case options.Command.IncludeNameMentions && message.MentionedExplicitly && message.MentionedByAlias:
				matchType = "explicit+alias"
			case message.MentionedExplicitly:
				matchType = "explicit"
			case options.Command.IncludeNameMentions && message.MentionedByAlias:
				matchType = "alias"
			default:
				continue
			}

			hits = append(hits, mentionHit{
				Timestamp:            message.Timestamp,
				Sender:               message.SenderDisplayName,
				ConversationType:     message.ConversationType,
				ConversationTitle:    descriptor.Target.Title,
				TeamID:               message.TeamID,
				ChannelID:            message.ChannelID,
				ChatID:               message.ChatID,
				MessageID:            message.MessageID,
				ThreadRootSequenceID: message.ThreadRootSequenceID,
				MatchType:            matchType,
				MessageText:          renderableMessageText(message),
				Preview:              previewSnippet(renderableMessageText(message)),
			})
			if options.Command.ResultLimit > 0 && len(hits) >= options.Command.ResultLimit {
				return hits, nil
			}
		}
	}

	if len(hits) == 0 && failureCount == len(descriptors) && lastErr != nil {
		return nil, lastErr
	}

	return hits, nil
}

func collectSearchHits(ctx context.Context, env *commandEnvironment, options AppOptions) ([]searchHit, error) {
	since, err := parseSinceValue(options.Command.Since)
	if err != nil {
		return nil, err
	}

	query := strings.ToLower(strings.TrimSpace(options.Command.Query))
	descriptors := prioritizeConversationDescriptors(filterConversationDescriptors(env.conversations, env.selectors, ""), since)
	hits := make([]searchHit, 0, options.Command.ResultLimit)
	failureCount := 0
	var lastErr error
	for _, descriptor := range descriptors {
		rawMessages, err := env.service.fetchMessageHistory(ctx, descriptor.Target, messageFetchOptions{
			PageSize: defaultMessageLimit,
			Since:    since,
		})
		if err != nil {
			if shouldSkipConversationScanError(descriptors, err) {
				failureCount++
				lastErr = err
				logSkippedConversationFetch(env, descriptor, err)
				continue
			}
			return nil, err
		}
		normalized := normalizeConversationMessages(descriptor, rawMessages, env.teamsState.me, env.selectors.MentionAliases)
		for _, message := range normalized {
			corpus := strings.ToLower(strings.Join([]string{message.ContentText, message.Subject, message.Title}, " "))
			if !strings.Contains(corpus, query) {
				continue
			}
			hits = append(hits, searchHit{
				Timestamp:         message.Timestamp,
				ConversationType:  message.ConversationType,
				ConversationTitle: descriptor.Target.Title,
				MessageID:         message.MessageID,
				Sender:            message.SenderDisplayName,
				MessageText:       renderableMessageText(message),
				Preview:           previewSnippet(renderableMessageText(message)),
			})
			if options.Command.ResultLimit > 0 && len(hits) >= options.Command.ResultLimit {
				return hits, nil
			}
		}
	}

	if len(hits) == 0 && failureCount == len(descriptors) && lastErr != nil {
		return nil, lastErr
	}

	return hits, nil
}

func collectWatchMentionHits(ctx context.Context, env *commandEnvironment, options AppOptions, stateFile string) ([]mentionHit, watchCheckpoint, error) {
	checkpoints, err := loadWatchState(stateFile)
	if err != nil {
		return nil, watchCheckpoint{}, err
	}
	key := selectorStateKey(env.selectors)
	checkpoint := checkpoints.Checkpoints[key]

	since, err := parseSinceValue(options.Command.Since)
	if err != nil {
		return nil, watchCheckpoint{}, err
	}
	if checkpoint.LastTimestamp != "" {
		parsed, err := time.Parse(time.RFC3339Nano, checkpoint.LastTimestamp)
		if err == nil {
			since = &parsed
		}
	}
	if since == nil {
		defaultSince := time.Now().Add(-15 * time.Minute)
		since = &defaultSince
	}

	mutable := options
	mutable.Command.Since = since.Format(time.RFC3339)
	hits, err := collectMentionHits(ctx, env, mutable)
	if err != nil {
		return nil, watchCheckpoint{}, err
	}

	filtered := make([]mentionHit, 0, len(hits))
	for _, hit := range hits {
		if checkpoint.LastTimestamp != "" {
			lastTime, err := time.Parse(time.RFC3339Nano, checkpoint.LastTimestamp)
			if err == nil {
				if hit.Timestamp.Before(lastTime) {
					continue
				}
				if hit.Timestamp.Equal(lastTime) && hit.MessageID == checkpoint.LastMessageID {
					continue
				}
			}
		}
		filtered = append(filtered, hit)
	}

	if len(filtered) == 0 {
		return nil, checkpoint, nil
	}
	last := filtered[len(filtered)-1]
	return filtered, watchCheckpoint{
		LastTimestamp: last.Timestamp.Format(time.RFC3339Nano),
		LastMessageID: last.MessageID,
	}, nil
}

func resolveConversationTarget(env *commandEnvironment, mode CommandMode, targetID string) (conversationDescriptor, error) {
	wantType := conversationTypeChannel
	if mode == commandModeReadChat || mode == commandModeExportChat {
		wantType = conversationTypeChat
	}

	for _, descriptor := range env.conversations {
		if descriptor.Target.ID == targetID && descriptor.Type == wantType {
			return descriptor, nil
		}
	}

	return conversationDescriptor{
		Target: ConversationTarget{
			ID:    targetID,
			Title: targetID,
		},
		Type:      wantType,
		ChannelID: targetIDIf(wantType == conversationTypeChannel, targetID),
		ChatID:    targetIDIf(wantType == conversationTypeChat, targetID),
	}, nil
}

func shouldSkipConversationScanError(descriptors []conversationDescriptor, err error) bool {
	if len(descriptors) <= 1 {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}

	var statusErr *messageFetchStatusError
	if !errors.As(err, &statusErr) {
		return false
	}
	if statusErr.StatusCode != http.StatusNotFound {
		return false
	}

	body := strings.ToLower(strings.TrimSpace(statusErr.Body))
	return strings.Contains(body, "softdeleted") || strings.Contains(body, "thread was not found")
}

func logSkippedConversationFetch(env *commandEnvironment, descriptor conversationDescriptor, err error) {
	if env == nil || env.service.logger == nil {
		return
	}

	env.service.logger.WithFields(logrus.Fields{
		"conversation_id":    descriptor.Target.ID,
		"conversation_title": descriptor.Target.Title,
		"conversation_type":  descriptor.Type,
	}).WithError(err).Warn("skipping conversation after fetch failure")
}

func targetIDIf(ok bool, value string) string {
	if ok {
		return value
	}

	return ""
}

func filterConversationDescriptors(descriptors []conversationDescriptor, selectors CommandSelectors, requiredType string) []conversationDescriptor {
	teamFilter := makeSet(selectors.TeamIDs)
	channelFilter := makeSet(selectors.ChannelIDs)
	chatFilter := makeSet(selectors.ChatIDs)

	filtered := make([]conversationDescriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if requiredType != "" && descriptor.Type != requiredType {
			continue
		}
		if len(channelFilter) > 0 && descriptor.Type != conversationTypeChannel {
			continue
		}
		if len(chatFilter) > 0 && descriptor.Type != conversationTypeChat {
			continue
		}
		if len(teamFilter) > 0 && descriptor.Type != conversationTypeChannel && len(channelFilter) == 0 && len(chatFilter) == 0 {
			continue
		}
		if len(teamFilter) > 0 && descriptor.TeamID != "" {
			if _, ok := teamFilter[descriptor.TeamID]; !ok {
				continue
			}
		}
		if descriptor.Type == conversationTypeChannel && len(channelFilter) > 0 {
			if _, ok := channelFilter[descriptor.ChannelID]; !ok {
				continue
			}
		}
		if descriptor.Type == conversationTypeChat && len(chatFilter) > 0 {
			if _, ok := chatFilter[descriptor.ChatID]; !ok {
				continue
			}
		}
		filtered = append(filtered, descriptor)
	}

	return filtered
}

func prioritizeConversationDescriptors(descriptors []conversationDescriptor, since *time.Time) []conversationDescriptor {
	filtered := make([]conversationDescriptor, 0, len(descriptors))
	for _, descriptor := range descriptors {
		if since != nil && !descriptor.LastActivity.IsZero() && descriptor.LastActivity.Before(*since) {
			continue
		}
		filtered = append(filtered, descriptor)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := filtered[i].LastActivity
		right := filtered[j].LastActivity
		switch {
		case left.Equal(right):
			return filtered[i].Target.Title < filtered[j].Target.Title
		case left.IsZero():
			return false
		case right.IsZero():
			return true
		default:
			return left.After(right)
		}
	})

	return filtered
}

func renderThreadedText(out io.Writer, descriptor conversationDescriptor, messages []normalizedMessage) error {
	if _, err := fmt.Fprintf(out, "%s\n", descriptor.Target.Title); err != nil {
		return err
	}
	for _, message := range messages {
		indent := ""
		if message.ParentSequenceID > 0 {
			indent = "  "
		}
		if _, err := fmt.Fprintf(out, "%s%s %s [%s]\n", indent, message.Timestamp.Format(time.RFC3339), message.SenderDisplayName, message.MessageID); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "%s%s\n", indent, message.ContentText); err != nil {
			return err
		}
	}

	return nil
}

func renderThreadedMarkdown(out io.Writer, descriptor conversationDescriptor, messages []normalizedMessage) error {
	if _, err := fmt.Fprintf(out, "# %s\n\n", descriptor.Target.Title); err != nil {
		return err
	}
	for _, message := range messages {
		prefix := "##"
		if message.ParentSequenceID > 0 {
			prefix = "###"
		}
		if _, err := fmt.Fprintf(out, "%s %s %s (`%s`)\n\n%s\n\n", prefix, message.Timestamp.Format(time.RFC3339), message.SenderDisplayName, message.MessageID, message.ContentText); err != nil {
			return err
		}
	}

	return nil
}

func renderMentionHitsText(out io.Writer, hits []mentionHit) error {
	for _, hit := range hits {
		if _, err := fmt.Fprintf(out, "%s %s %s %s [%s]\n", hit.Timestamp.Format(time.RFC3339), hit.Sender, hit.MatchType, hit.ConversationTitle, hit.MessageID); err != nil {
			return err
		}
		if err := writeIndentedBody(out, hit.MessageText); err != nil {
			return err
		}
	}
	return nil
}

func renderSearchHitsText(out io.Writer, hits []searchHit) error {
	for _, hit := range hits {
		if _, err := fmt.Fprintf(out, "%s %s %s %s [%s]\n", hit.Timestamp.Format(time.RFC3339), hit.ConversationType, hit.ConversationTitle, hit.Sender, hit.MessageID); err != nil {
			return err
		}
		if err := writeIndentedBody(out, hit.MessageText); err != nil {
			return err
		}
	}
	return nil
}

func writeIndentedBody(out io.Writer, body string) error {
	text := strings.TrimSpace(body)
	if text == "" {
		text = "(empty message)"
	}
	for _, line := range strings.Split(text, "\n") {
		if _, err := fmt.Fprintf(out, "  %s\n", line); err != nil {
			return err
		}
	}
	return nil
}

func renderableMessageText(message normalizedMessage) string {
	for _, candidate := range []string{message.ContentText, message.Subject, message.Title} {
		candidate = strings.TrimSpace(candidate)
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func previewSnippet(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	if len(text) <= 120 {
		return text
	}
	return text[:117] + "..."
}

func writeJSON(out io.Writer, value any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func writeCommandOutput(out io.Writer, outputPath, data string) error {
	if strings.TrimSpace(outputPath) == "" {
		_, err := io.WriteString(out, data)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(outputPath, []byte(data), 0o600)
}

func parseSinceValue(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	if duration, err := parseFlexibleDuration(raw); err == nil {
		since := time.Now().Add(-duration)
		return &since, nil
	}

	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("invalid since value %q", raw)
}

func resolveWatchStateFile(options AppOptions, selectors CommandSelectors) (string, error) {
	if strings.TrimSpace(options.Command.StateFile) != "" {
		return options.Command.StateFile, nil
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".local", "state", "teams-cli", "watch-mentions.json"), nil
}

func selectorStateKey(selectors CommandSelectors) string {
	parts := []string{
		strings.Join(selectors.TeamIDs, ","),
		strings.Join(selectors.ChannelIDs, ","),
		strings.Join(selectors.ChatIDs, ","),
		strings.Join(selectors.MentionAliases, ","),
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func loadWatchState(path string) (watchStateFile, error) {
	state := watchStateFile{Checkpoints: map[string]watchCheckpoint{}}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, err
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return state, err
	}
	if err := json.Unmarshal(body, &state); err != nil {
		return state, err
	}
	if state.Checkpoints == nil {
		state.Checkpoints = map[string]watchCheckpoint{}
	}

	return state, nil
}

func persistWatchCheckpoint(path, key string, checkpoint watchCheckpoint) error {
	state, err := loadWatchState(path)
	if err != nil {
		return err
	}
	state.Checkpoints[key] = checkpoint
	body, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o600)
}

func makeSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}

	return set
}
