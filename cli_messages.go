package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	teams_api "github.com/saimon-moore/teams-api"
	"github.com/saimon-moore/teams-api/pkg/csa"
	"github.com/saimon-moore/teams-api/pkg/models"
	"github.com/sirupsen/logrus"
)

const (
	conversationTypeChannel = "channel"
	conversationTypeChat    = "chat"
)

type conversationDescriptor struct {
	Target       ConversationTarget
	Type         string
	TeamID       string
	TeamName     string
	ChannelID    string
	ChannelName string
	ChatID       string
	LastActivity time.Time
}

type messageFetchOptions struct {
	PageSize    int
	MaxMessages int
	Since       *time.Time
}

type normalizedMessage struct {
	MessageID            string    `json:"message_id"`
	SequenceID           int64     `json:"sequence_id"`
	ConversationID       string    `json:"conversation_id"`
	ConversationType     string    `json:"conversation_type"`
	TeamID               string    `json:"team_id,omitempty"`
	ChannelID            string    `json:"channel_id,omitempty"`
	ChatID               string    `json:"chat_id,omitempty"`
	SenderDisplayName    string    `json:"sender_display_name"`
	SenderID             string    `json:"sender_id"`
	Timestamp            time.Time `json:"timestamp"`
	ContentText          string    `json:"content_text"`
	ContentHTML          string    `json:"content_html"`
	Subject              string    `json:"subject,omitempty"`
	Title                string    `json:"title,omitempty"`
	MentionsRaw          string    `json:"mentions_raw,omitempty"`
	MentionedExplicitly  bool      `json:"mentioned_explicitly"`
	MentionedByAlias     bool      `json:"mentioned_by_alias"`
	ParentSequenceID     int64     `json:"parent_sequence_id,omitempty"`
	ParentMessageID      string    `json:"parent_message_id,omitempty"`
	ThreadRootSequenceID int64     `json:"thread_root_sequence_id"`
}

type commandService struct {
	teamsClient *teams_api.TeamsClient
	logger      *logrus.Logger
	httpClient  *http.Client
}

func (s commandService) fetchMessageHistory(ctx context.Context, target ConversationTarget, opts messageFetchOptions) ([]csa.ChatMessage, error) {
	if s.teamsClient == nil {
		return nil, fmt.Errorf("teams client is nil")
	}

	pageSize := opts.PageSize
	if pageSize <= 0 {
		pageSize = defaultMessageLimit
	}

	baseURL, err := url.Parse(csa.MessagesHost)
	if err != nil {
		return nil, fmt.Errorf("unable to parse messages host: %v", err)
	}

	endpointURL, err := baseURL.Parse("v1/users/ME/conversations/" + url.QueryEscape(target.ID) + "/messages")
	if err != nil {
		return nil, fmt.Errorf("unable to parse messages endpoint: %v", err)
	}

	values := endpointURL.Query()
	values.Add("view", "msnp24Equivalent|supportsMessageProperties")
	values.Add("pageSize", fmt.Sprintf("%d", pageSize))
	values.Add("startTime", "1")
	endpointURL.RawQuery = values.Encode()

	nextEndpoint := endpointURL.String()
	collected := make([]csa.ChatMessage, 0, pageSize)

	for nextEndpoint != "" {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		response, err := s.fetchMessagesResponse(ctx, nextEndpoint)
		if err != nil {
			return nil, err
		}

		pageMessages := response.Messages
		sort.Sort(csa.SortMessageByTime(pageMessages))
		collected = append(collected, pageMessages...)

		if opts.Since != nil && len(pageMessages) > 0 {
			oldest := time.Time(pageMessages[0].ComposeTime)
			if oldest.Before(*opts.Since) || oldest.Equal(*opts.Since) {
				break
			}
		}

		if opts.MaxMessages > 0 && len(collected) >= opts.MaxMessages {
			break
		}

		nextEndpoint = strings.TrimSpace(response.Metadata.BackwardLink)
	}

	sort.Sort(csa.SortMessageByTime(collected))
	if opts.Since != nil {
		filtered := collected[:0]
		for _, message := range collected {
			ts := time.Time(message.ComposeTime)
			if ts.Before(*opts.Since) {
				continue
			}
			filtered = append(filtered, message)
		}
		collected = filtered
	}
	if opts.MaxMessages > 0 && len(collected) > opts.MaxMessages {
		collected = collected[len(collected)-opts.MaxMessages:]
	}

	return collected, nil
}

func (s commandService) fetchMessagesResponse(ctx context.Context, endpoint string) (csa.MessagesResponse, error) {
	var zero csa.MessagesResponse
	var lastErr error

	for attempt := 1; attempt <= messageFetchMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return zero, err
		}

		attemptCtx, cancel := context.WithTimeout(ctx, messageRequestTimeout)
		response, err := s.fetchMessagesResponseOnce(attemptCtx, endpoint)
		cancel()
		if err == nil {
			return response, nil
		}
		if errors.Is(err, context.Canceled) {
			return zero, err
		}
		lastErr = err
		if attempt == messageFetchMaxAttempts || !shouldRetryMessageFetch(err) {
			return zero, err
		}

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(messageFetchBackoff(attempt)):
		}
	}

	return zero, lastErr
}

func (s commandService) fetchMessagesResponseOnce(ctx context.Context, endpoint string) (csa.MessagesResponse, error) {
	var zero csa.MessagesResponse
	req, err := s.teamsClient.ChatSvc().AuthenticatedRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return zero, err
	}
	req = req.WithContext(ctx)

	client := s.httpClient
	if client == nil {
		client = newMessageHTTPClient()
	}

	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return zero, &messageFetchStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       strings.TrimSpace(string(bodyBytes)),
		}
	}

	var msgResponse csa.MessagesResponse
	if err := json.NewDecoder(resp.Body).Decode(&msgResponse); err != nil {
		return zero, fmt.Errorf("unable to decode messages: %v", err)
	}

	return msgResponse, nil
}

func normalizeConversationMessages(conversation conversationDescriptor, messages []csa.ChatMessage, me *models.User, extraAliases []string) []normalizedMessage {
	aliases := mentionAliases(me, extraAliases)
	normalized := make([]normalizedMessage, 0, len(messages))
	sequenceIndex := map[int64]int{}

	for _, message := range messages {
		contentText := normalizeMessageText(message.Content)
		normalized = append(normalized, normalizedMessage{
			MessageID:           message.Id,
			SequenceID:          message.SequenceId,
			ConversationID:      conversation.Target.ID,
			ConversationType:    conversation.Type,
			TeamID:              conversation.TeamID,
			ChannelID:           conversation.ChannelID,
			ChatID:              conversation.ChatID,
			SenderDisplayName:   message.ImDisplayName,
			SenderID:            message.From,
			Timestamp:           time.Time(message.ComposeTime),
			ContentText:         contentText,
			ContentHTML:         message.Content,
			Subject:             strings.TrimSpace(message.Properties.Subject),
			Title:               strings.TrimSpace(message.Properties.Title),
			MentionsRaw:         strings.TrimSpace(message.Properties.Mentions),
			MentionedExplicitly: mentionsCurrentUser(message.Properties.Mentions, me),
			MentionedByAlias:    containsAnyFold(contentText, aliases),
			ParentSequenceID:    message.Properties.ParentMessageId,
		})
		sequenceIndex[message.SequenceId] = len(normalized) - 1
	}

	for idx := range normalized {
		parentSeq := normalized[idx].ParentSequenceID
		if parentSeq <= 0 {
			normalized[idx].ThreadRootSequenceID = normalized[idx].SequenceID
			continue
		}

		if parentIdx, ok := sequenceIndex[parentSeq]; ok {
			normalized[idx].ParentMessageID = normalized[parentIdx].MessageID
			rootSeq := normalized[parentIdx].ThreadRootSequenceID
			if rootSeq == 0 {
				rootSeq = normalized[parentIdx].SequenceID
			}
			normalized[idx].ThreadRootSequenceID = rootSeq
			continue
		}

		normalized[idx].ThreadRootSequenceID = normalized[idx].SequenceID
	}

	for idx := range normalized {
		if normalized[idx].ThreadRootSequenceID == 0 {
			normalized[idx].ThreadRootSequenceID = normalized[idx].SequenceID
		}
	}

	return normalized
}

func normalizeMessageText(raw string) string {
	text := strings.TrimSpace(textMessage(raw))
	if text != "" {
		return strings.Join(strings.Fields(text), " ")
	}

	return strings.TrimSpace(raw)
}

func mentionAliases(me *models.User, extraAliases []string) []string {
	aliases := []string{}
	if me != nil {
		aliases = appendUniqueNormalized(aliases,
			me.DisplayName,
			me.Email,
			me.Mail,
			me.UserPrincipalName,
			me.Alias,
		)
	}

	return appendUniqueNormalized(aliases, extraAliases...)
}

func mentionsCurrentUser(raw string, me *models.User) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}

	identifiers := []string{}
	if me != nil {
		identifiers = append(identifiers, me.Mri, me.ObjectId, me.Email, me.Mail, me.UserPrincipalName, me.Alias)
	}

	return containsAnyFold(raw, identifiers)
}

func containsAnyFold(haystack string, needles []string) bool {
	haystack = strings.ToLower(haystack)
	for _, needle := range needles {
		needle = strings.TrimSpace(needle)
		if needle == "" {
			continue
		}
		if strings.Contains(haystack, strings.ToLower(needle)) {
			return true
		}
	}

	return false
}

func buildConversationDescriptors(state *TeamsState) []conversationDescriptor {
	if state == nil || state.conversations == nil {
		return nil
	}

	descriptors := make([]conversationDescriptor, 0, len(state.conversations.Teams)+len(state.conversations.Chats))
	for _, team := range state.conversations.Teams {
		for _, channel := range team.Channels {
			descriptors = append(descriptors, conversationDescriptor{
				Target: ConversationTarget{
					ID:    channel.Id,
					Title: team.DisplayName + " / " + channel.DisplayName,
				},
				Type:         conversationTypeChannel,
				TeamID:       team.Id,
				TeamName:     team.DisplayName,
				ChannelID:    channel.Id,
				ChannelName: channel.DisplayName,
				LastActivity: channelActivityTime(channel),
			})
		}
	}

	for _, chat := range state.conversations.Chats {
		title := resolveChatTitle(chat, state.selfMri(), state.selfDisplayName())
		descriptors = append(descriptors, conversationDescriptor{
			Target: ConversationTarget{
				ID:    chat.Id,
				Title: title,
			},
			Type:         conversationTypeChat,
			ChatID:       chat.Id,
			LastActivity: chatActivityTime(chat),
		})
	}

	return descriptors
}

func channelActivityTime(channel csa.Channel) time.Time {
	if ts := time.Time(channel.LastMessage.ComposeTime); !ts.IsZero() {
		return ts
	}
	if ts := time.Time(channel.LastMessage.OriginalArrivalTime); !ts.IsZero() {
		return ts
	}
	if !channel.LastJoinAt.IsZero() {
		return channel.LastJoinAt
	}
	if !channel.LastLeaveAt.IsZero() {
		return channel.LastLeaveAt
	}
	if !channel.CreationTime.IsZero() {
		return channel.CreationTime
	}

	return time.Time{}
}
