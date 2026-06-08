package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	api "github.com/saimon-moore/teams-api/pkg"
	"github.com/saimon-moore/teams-api/pkg/csa"
	"github.com/saimon-moore/teams-api/pkg/models"
)

func TestCommandServiceFetchMessageHistoryFollowsBackwardLinkUntilSince(t *testing.T) {
	since := time.Date(2026, time.March, 27, 9, 1, 30, 0, time.UTC)
	var requestCount int

	service := commandService{
		teamsClient: newTestTeamsClient(t),
		logger:      discardLogger,
		httpClient: &http.Client{
			Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
				requestCount++

				switch requestCount {
				case 1:
					body := testMessagesPageJSON(t, []csa.ChatMessage{
						testSequencedChatMessage("message-3", 3, 0, "Carol", "third", time.Date(2026, time.March, 27, 9, 3, 0, 0, time.UTC)),
						testSequencedChatMessage("message-2", 2, 1, "Bob", "second", time.Date(2026, time.March, 27, 9, 2, 0, 0, time.UTC)),
					}, "https://example.invalid/page-2")
					return testHTTPResponse(http.StatusOK, body), nil
				case 2:
					if req.URL.String() != "https://example.invalid/page-2" {
						t.Fatalf("expected backward link to be followed, got %q", req.URL.String())
					}
					body := testMessagesPageJSON(t, []csa.ChatMessage{
						testSequencedChatMessage("message-1", 1, 0, "Alice", "first", time.Date(2026, time.March, 27, 9, 1, 0, 0, time.UTC)),
					}, "")
					return testHTTPResponse(http.StatusOK, body), nil
				default:
					t.Fatalf("unexpected request count %d", requestCount)
					return nil, nil
				}
			}),
		},
	}

	messages, err := service.fetchMessageHistory(context.Background(), ConversationTarget{
		ID:    "19:channel",
		Title: "Engineering / General",
	}, messageFetchOptions{
		PageSize:    200,
		MaxMessages: 10,
		Since:       &since,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if requestCount != 2 {
		t.Fatalf("expected 2 page requests, got %d", requestCount)
	}
	if len(messages) != 2 {
		t.Fatalf("expected messages older than since to be filtered out, got %d", len(messages))
	}
	if messages[0].Id != "message-2" || messages[1].Id != "message-3" {
		t.Fatalf("expected filtered messages 2 and 3, got %q then %q", messages[0].Id, messages[1].Id)
	}
}

func TestNormalizeConversationMessagesResolvesThreadsAndMentions(t *testing.T) {
	conversation := conversationDescriptor{
		Target:    ConversationTarget{ID: "19:channel", Title: "Engineering / General"},
		Type:      conversationTypeChannel,
		TeamID:    "team-1",
		ChannelID: "19:channel",
	}
	me := &models.User{
		DisplayName:       "Saimon Moore",
		Email:             "saimon@example.com",
		Alias:             "saimon",
		Mri:               "8:orgid:saimon",
		UserPrincipalName: "saimon@example.com",
	}
	messages := []csa.ChatMessage{
		{
			Id:            "root",
			SequenceId:    10,
			ImDisplayName: "Alice",
			From:          "8:alice",
			Content:       "<p>Hello Saimon</p>",
			ComposeTime:   api.RFC3339Time(time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)),
			Properties: csa.ChatMessageProperties{
				Mentions: `[{"mri":"8:orgid:saimon"}]`,
				Title:    "Incident",
			},
		},
		{
			Id:            "reply",
			SequenceId:    11,
			ImDisplayName: "Bob",
			From:          "8:bob",
			Content:       "<p>reply</p>",
			ComposeTime:   api.RFC3339Time(time.Date(2026, time.March, 27, 10, 1, 0, 0, time.UTC)),
			Properties: csa.ChatMessageProperties{
				ParentMessageId: 10,
			},
		},
	}

	normalized := normalizeConversationMessages(conversation, messages, me, []string{"@saimon"})
	if len(normalized) != 2 {
		t.Fatalf("expected 2 normalized messages, got %d", len(normalized))
	}

	if !normalized[0].MentionedExplicitly {
		t.Fatal("expected explicit mention to be detected")
	}
	if !normalized[0].MentionedByAlias {
		t.Fatal("expected alias mention to be detected from message text")
	}
	if normalized[1].ParentSequenceID != 10 {
		t.Fatalf("expected reply parent sequence to be retained, got %d", normalized[1].ParentSequenceID)
	}
	if normalized[1].ParentMessageID != "root" {
		t.Fatalf("expected reply parent message id to resolve, got %q", normalized[1].ParentMessageID)
	}
	if normalized[1].ThreadRootSequenceID != 10 {
		t.Fatalf("expected reply thread root sequence to resolve, got %d", normalized[1].ThreadRootSequenceID)
	}
}

func TestFilterConversationDescriptorsKeepsChannelSelectorsScopedToChannels(t *testing.T) {
	descriptors := []conversationDescriptor{
		{Type: conversationTypeChannel, ChannelID: "channel-1", TeamID: "team-1"},
		{Type: conversationTypeChat, ChatID: "chat-1"},
	}

	filtered := filterConversationDescriptors(descriptors, CommandSelectors{
		ChannelIDs: []string{"channel-1"},
	}, "")

	if len(filtered) != 1 || filtered[0].Type != conversationTypeChannel {
		t.Fatalf("expected only the selected channel descriptor, got %#v", filtered)
	}
}

func TestCollectSearchHitsSkipsSoftDeletedConversationErrors(t *testing.T) {
	env := &commandEnvironment{
		service: commandService{
			teamsClient: newTestTeamsClient(t),
			logger:      discardLogger,
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case strings.Contains(req.URL.Path, "/19:soft/messages"):
						return testHTTPResponse(http.StatusNotFound, `{"standardizedError":{"errorDescription":"SoftDeleted-Thread was not found as it is soft-deleted."}}`), nil
					case strings.Contains(req.URL.Path, "/19:good/messages"):
						return testHTTPResponse(http.StatusOK, testMessagesPageJSON(t, []csa.ChatMessage{
							testSequencedChatMessage("message-1", 1, 0, "Alice", "<p>ucl result</p>", time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)),
						}, "")), nil
					default:
						t.Fatalf("unexpected fetch path %q", req.URL.Path)
						return nil, nil
					}
				}),
			},
		},
		teamsState: TeamsState{
			me: &models.User{DisplayName: "Me"},
		},
		conversations: []conversationDescriptor{
			{Target: ConversationTarget{ID: "19:soft", Title: "Soft Deleted"}, Type: conversationTypeChat, ChatID: "19:soft"},
			{Target: ConversationTarget{ID: "19:good", Title: "Good Chat"}, Type: conversationTypeChat, ChatID: "19:good"},
		},
	}

	hits, err := collectSearchHits(context.Background(), env, AppOptions{
		Command: CommandOptions{
			Query:       "ucl",
			ResultLimit: 10,
		},
	})
	if err != nil {
		t.Fatalf("expected soft-deleted conversations to be skipped, got %v", err)
	}
	if len(hits) != 1 || hits[0].ConversationTitle != "Good Chat" {
		t.Fatalf("expected only the good chat hit, got %#v", hits)
	}
}

func TestCollectMentionHitsSkipsSoftDeletedConversationErrors(t *testing.T) {
	env := &commandEnvironment{
		service: commandService{
			teamsClient: newTestTeamsClient(t),
			logger:      discardLogger,
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case strings.Contains(req.URL.Path, "/19:soft/messages"):
						return testHTTPResponse(http.StatusNotFound, `{"standardizedError":{"errorDescription":"SoftDeleted-Thread was not found as it is soft-deleted."}}`), nil
					case strings.Contains(req.URL.Path, "/19:good/messages"):
						return testHTTPResponse(http.StatusOK, testMessagesPageJSON(t, []csa.ChatMessage{
							{
								Id:            "message-1",
								SequenceId:    1,
								ImDisplayName: "Alice",
								From:          "8:alice",
								Content:       "<p>Hello Saimon</p>",
								ComposeTime:   api.RFC3339Time(time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)),
								Properties: csa.ChatMessageProperties{
									Mentions: `[{"mri":"8:orgid:saimon"}]`,
								},
							},
						}, "")), nil
					default:
						t.Fatalf("unexpected fetch path %q", req.URL.Path)
						return nil, nil
					}
				}),
			},
		},
		teamsState: TeamsState{
			me: &models.User{DisplayName: "Saimon", Mri: "8:orgid:saimon"},
		},
		conversations: []conversationDescriptor{
			{Target: ConversationTarget{ID: "19:soft", Title: "Soft Deleted"}, Type: conversationTypeChat, ChatID: "19:soft"},
			{Target: ConversationTarget{ID: "19:good", Title: "Good Chat"}, Type: conversationTypeChat, ChatID: "19:good"},
		},
	}

	hits, err := collectMentionHits(context.Background(), env, AppOptions{
		Command: CommandOptions{
			ResultLimit: 10,
		},
	})
	if err != nil {
		t.Fatalf("expected soft-deleted conversations to be skipped, got %v", err)
	}
	if len(hits) != 1 || hits[0].ConversationTitle != "Good Chat" {
		t.Fatalf("expected only the good chat hit, got %#v", hits)
	}
}

func TestCollectMentionHitsDefaultsToExplicitMentionsOnly(t *testing.T) {
	env := &commandEnvironment{
		service: commandService{
			teamsClient: newTestTeamsClient(t),
			logger:      discardLogger,
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					return testHTTPResponse(http.StatusOK, testMessagesPageJSON(t, []csa.ChatMessage{
						{
							Id:            "alias-only",
							SequenceId:    1,
							ImDisplayName: "Alice",
							From:          "8:alice",
							Content:       "<p>Hello Saimon</p>",
							ComposeTime:   api.RFC3339Time(time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)),
						},
					}, "")), nil
				}),
			},
		},
		teamsState: TeamsState{
			me: &models.User{DisplayName: "Saimon", Mri: "8:orgid:saimon"},
		},
		conversations: []conversationDescriptor{
			{Target: ConversationTarget{ID: "19:good", Title: "Good Chat"}, Type: conversationTypeChat, ChatID: "19:good"},
		},
		selectors: CommandSelectors{
			MentionAliases: []string{"Saimon"},
		},
	}

	hits, err := collectMentionHits(context.Background(), env, AppOptions{
		Command: CommandOptions{
			ResultLimit: 10,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected alias-only matches to be excluded by default, got %#v", hits)
	}
}

func TestCollectMentionHitsCanIncludeNameMentions(t *testing.T) {
	env := &commandEnvironment{
		service: commandService{
			teamsClient: newTestTeamsClient(t),
			logger:      discardLogger,
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					return testHTTPResponse(http.StatusOK, testMessagesPageJSON(t, []csa.ChatMessage{
						{
							Id:            "alias-only",
							SequenceId:    1,
							ImDisplayName: "Alice",
							From:          "8:alice",
							Content:       "<p>Hello Saimon</p>",
							ComposeTime:   api.RFC3339Time(time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)),
						},
					}, "")), nil
				}),
			},
		},
		teamsState: TeamsState{
			me: &models.User{DisplayName: "Saimon", Mri: "8:orgid:saimon"},
		},
		conversations: []conversationDescriptor{
			{Target: ConversationTarget{ID: "19:good", Title: "Good Chat"}, Type: conversationTypeChat, ChatID: "19:good"},
		},
		selectors: CommandSelectors{
			MentionAliases: []string{"Saimon"},
		},
	}

	hits, err := collectMentionHits(context.Background(), env, AppOptions{
		Command: CommandOptions{
			ResultLimit:         10,
			IncludeNameMentions: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 || hits[0].MatchType != "alias" {
		t.Fatalf("expected alias match when include-name-mentions is enabled, got %#v", hits)
	}
	if hits[0].MessageText != "Hello Saimon" {
		t.Fatalf("expected full mention message text to be retained, got %q", hits[0].MessageText)
	}
}

func TestResolveConversationTargetAllowsExplicitChatFallback(t *testing.T) {
	env := &commandEnvironment{
		conversations: []conversationDescriptor{},
	}

	descriptor, err := resolveConversationTarget(env, commandModeReadChat, "19:missing-chat")
	if err != nil {
		t.Fatalf("expected explicit chat ids to be accepted, got %v", err)
	}
	if descriptor.Type != conversationTypeChat || descriptor.ChatID != "19:missing-chat" || descriptor.Target.ID != "19:missing-chat" {
		t.Fatalf("unexpected fallback descriptor %#v", descriptor)
	}
}

func TestCollectSearchHitsSkipsConversationTimeouts(t *testing.T) {
	env := &commandEnvironment{
		service: commandService{
			teamsClient: newTestTeamsClient(t),
			logger:      discardLogger,
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case strings.Contains(req.URL.Path, "/19:slow/messages"):
						return nil, context.DeadlineExceeded
					case strings.Contains(req.URL.Path, "/19:good/messages"):
						return testHTTPResponse(http.StatusOK, testMessagesPageJSON(t, []csa.ChatMessage{
							testSequencedChatMessage("message-1", 1, 0, "Alice", "<p>ucl result</p>", time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)),
						}, "")), nil
					default:
						t.Fatalf("unexpected fetch path %q", req.URL.Path)
						return nil, nil
					}
				}),
			},
		},
		teamsState: TeamsState{
			me: &models.User{DisplayName: "Me"},
		},
		conversations: []conversationDescriptor{
			{Target: ConversationTarget{ID: "19:slow", Title: "Slow Chat"}, Type: conversationTypeChat, ChatID: "19:slow"},
			{Target: ConversationTarget{ID: "19:good", Title: "Good Chat"}, Type: conversationTypeChat, ChatID: "19:good"},
		},
	}

	hits, err := collectSearchHits(context.Background(), env, AppOptions{
		Command: CommandOptions{
			Query:       "ucl",
			ResultLimit: 10,
		},
	})
	if err != nil {
		t.Fatalf("expected timed out conversations to be skipped, got %v", err)
	}
	if len(hits) != 1 || hits[0].ConversationTitle != "Good Chat" {
		t.Fatalf("expected only the good chat hit, got %#v", hits)
	}
}

func TestCollectSearchHitsSkipsConversationsWithStaleKnownActivity(t *testing.T) {
	since := time.Date(2026, time.March, 27, 9, 0, 0, 0, time.UTC)
	env := &commandEnvironment{
		service: commandService{
			teamsClient: newTestTeamsClient(t),
			logger:      discardLogger,
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					switch {
					case strings.Contains(req.URL.Path, "/19:stale/messages"):
						t.Fatal("stale conversation should have been skipped before fetch")
						return nil, nil
					case strings.Contains(req.URL.Path, "/19:fresh/messages"):
						return testHTTPResponse(http.StatusOK, testMessagesPageJSON(t, []csa.ChatMessage{
							testSequencedChatMessage("message-1", 1, 0, "Alice", "<p>ucl result</p>", time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)),
						}, "")), nil
					default:
						t.Fatalf("unexpected fetch path %q", req.URL.Path)
						return nil, nil
					}
				}),
			},
		},
		teamsState: TeamsState{
			me: &models.User{DisplayName: "Me"},
		},
		conversations: []conversationDescriptor{
			{
				Target:       ConversationTarget{ID: "19:stale", Title: "Stale Chat"},
				Type:         conversationTypeChat,
				ChatID:       "19:stale",
				LastActivity: since.Add(-2 * time.Hour),
			},
			{
				Target:       ConversationTarget{ID: "19:fresh", Title: "Fresh Chat"},
				Type:         conversationTypeChat,
				ChatID:       "19:fresh",
				LastActivity: since.Add(2 * time.Hour),
			},
		},
	}

	hits, err := collectSearchHits(context.Background(), env, AppOptions{
		Command: CommandOptions{
			Query:       "ucl",
			ResultLimit: 10,
			Since:       since.Format(time.RFC3339),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 || hits[0].ConversationTitle != "Fresh Chat" {
		t.Fatalf("expected only the fresh chat hit, got %#v", hits)
	}
}

func TestCollectSearchHitsIncludeFullMessageText(t *testing.T) {
	env := &commandEnvironment{
		service: commandService{
			teamsClient: newTestTeamsClient(t),
			logger:      discardLogger,
			httpClient: &http.Client{
				Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
					return testHTTPResponse(http.StatusOK, testMessagesPageJSON(t, []csa.ChatMessage{
						testSequencedChatMessage("message-1", 1, 0, "Alice", "<p>ucl result with more detail</p>", time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC)),
					}, "")), nil
				}),
			},
		},
		teamsState: TeamsState{
			me: &models.User{DisplayName: "Me"},
		},
		conversations: []conversationDescriptor{
			{Target: ConversationTarget{ID: "19:good", Title: "Good Chat"}, Type: conversationTypeChat, ChatID: "19:good"},
		},
	}

	hits, err := collectSearchHits(context.Background(), env, AppOptions{
		Command: CommandOptions{
			Query:       "ucl",
			ResultLimit: 10,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected one hit, got %#v", hits)
	}
	if hits[0].MessageText != "ucl result with more detail" {
		t.Fatalf("expected full search message text to be retained, got %q", hits[0].MessageText)
	}
}

func TestRenderMentionHitsTextIncludesMessageBody(t *testing.T) {
	var out bytes.Buffer
	err := renderMentionHitsText(&out, []mentionHit{
		{
			Timestamp:         time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC),
			Sender:            "Alice",
			MatchType:         "explicit",
			ConversationTitle: "Good Chat",
			MessageID:         "message-1",
			MessageText:       "Hello Saimon",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "Hello Saimon") {
		t.Fatalf("expected rendered output to include full message text, got %q", rendered)
	}
	if !strings.Contains(rendered, "message-1") {
		t.Fatalf("expected rendered output to include the message id, got %q", rendered)
	}
}

func TestRenderSearchHitsTextIncludesMessageBody(t *testing.T) {
	var out bytes.Buffer
	err := renderSearchHitsText(&out, []searchHit{
		{
			Timestamp:         time.Date(2026, time.March, 27, 10, 0, 0, 0, time.UTC),
			ConversationType:  conversationTypeChat,
			ConversationTitle: "Good Chat",
			MessageID:         "message-1",
			Sender:            "Alice",
			MessageText:       "ucl result with more detail",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "ucl result with more detail") {
		t.Fatalf("expected rendered output to include full message text, got %q", rendered)
	}
	if !strings.Contains(rendered, "message-1") {
		t.Fatalf("expected rendered output to include the message id, got %q", rendered)
	}
}

func testMessagesPageJSON(t *testing.T, messages []csa.ChatMessage, backwardLink string) string {
	t.Helper()

	bodyBytes, err := json.Marshal(csa.MessagesResponse{
		Messages: messages,
		Metadata: csa.MessagesMetadata{BackwardLink: backwardLink},
	})
	if err != nil {
		t.Fatalf("unable to marshal test messages page: %v", err)
	}

	return string(bodyBytes)
}

func testSequencedChatMessage(id string, sequenceID, parentSequenceID int64, author, content string, composedAt time.Time) csa.ChatMessage {
	return csa.ChatMessage{
		Id:            id,
		SequenceId:    sequenceID,
		Version:       "1",
		ImDisplayName: author,
		From:          "8:" + author,
		Content:       content,
		ComposeTime:   api.RFC3339Time(composedAt),
		Properties: csa.ChatMessageProperties{
			ParentMessageId: parentSequenceID,
		},
	}
}
