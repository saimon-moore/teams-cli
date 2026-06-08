package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const defaultMessageLimit = 200

type CommandMode string

const (
	commandModeTUI            CommandMode = ""
	commandModeListTeams      CommandMode = "list-teams"
	commandModeListCalendars  CommandMode = "list-calendars"
	commandModeListEvents     CommandMode = "list-events"
	commandModeListChannels   CommandMode = "list-channels"
	commandModeListMentions   CommandMode = "list-mentions"
	commandModeCreateEvent    CommandMode = "create-event"
	commandModeUpdateEvent    CommandMode = "update-event"
	commandModeDeleteEvent    CommandMode = "delete-event"
	commandModeReadChannel    CommandMode = "read-channel"
	commandModeReadChat       CommandMode = "read-chat"
	commandModeExportChannel  CommandMode = "export-channel"
	commandModeExportChat     CommandMode = "export-chat"
	commandModeWatchMentions  CommandMode = "watch-mentions"
	commandModeSearchMessages CommandMode = "search-messages"
)

type CommandOptions struct {
	TargetID            string
	CalendarID          string
	EventID             string
	OutputJSON          bool
	IncludeNameMentions bool
	Profile             string
	TeamIDs             []string
	ChannelIDs          []string
	ChatIDs             []string
	Since               string
	Start               string
	End                 string
	ResultLimit         int
	Query               string
	Subject             string
	Timezone            string
	Location            string
	Body                string
	AllDay              bool
	Format              string
	OutputPath          string
	PollInterval        time.Duration
	StateFile           string
	ConfigPath          string
}

type AppOptions struct {
	CommandMode                 CommandMode
	Command                     CommandOptions
	MessageLimit                int
	LogLevel                    logrus.Level
	TokenDir                    string
	LiveRefresh                 bool
	RefreshMessagesInterval     time.Duration
	RefreshConversationInterval time.Duration
	ShowHelp                    bool
	ShowVersion                 bool
	DoctorMode                  bool
}

func defaultAppOptions() AppOptions {
	return AppOptions{
		MessageLimit:                defaultMessageLimit,
		LogLevel:                    logrus.InfoLevel,
		LiveRefresh:                 true,
		RefreshMessagesInterval:     defaultLiveMessageRefreshInterval,
		RefreshConversationInterval: defaultLiveConversationRefreshInterval,
	}
}

func parseAppOptions(args []string) (AppOptions, error) {
	options := defaultAppOptions()

	for idx := 0; idx < len(args); idx++ {
		arg := args[idx]

		switch {
		case isCommandToken(arg):
			nextIdx, err := parseCommandOptions(args, idx, &options)
			if err != nil {
				return options, err
			}
			idx = nextIdx
		case arg == "doctor" || arg == "--doctor":
			options.DoctorMode = true
		case arg == "--help" || arg == "-h":
			options.ShowHelp = true
		case arg == "--version":
			options.ShowVersion = true
		case arg == "--debug":
			options.LogLevel = logrus.DebugLevel
		case arg == "--no-live":
			options.LiveRefresh = false
		case strings.HasPrefix(arg, "msg="):
			limit, err := parseMessageLimit(strings.TrimPrefix(arg, "msg="))
			if err != nil {
				return options, err
			}
			options.MessageLimit = limit
		case arg == "--msg" || strings.HasPrefix(arg, "--msg="):
			raw, nextIdx, err := optionValue(args, idx, "--msg")
			if err != nil {
				return options, err
			}
			limit, err := parseMessageLimit(raw)
			if err != nil {
				return options, err
			}
			options.MessageLimit = limit
			idx = nextIdx
		case arg == "--log-level" || strings.HasPrefix(arg, "--log-level="):
			raw, nextIdx, err := optionValue(args, idx, "--log-level")
			if err != nil {
				return options, err
			}
			level, err := logrus.ParseLevel(strings.ToLower(strings.TrimSpace(raw)))
			if err != nil {
				return options, fmt.Errorf("invalid log level %q", raw)
			}
			options.LogLevel = level
			idx = nextIdx
		case arg == "--token-dir" || strings.HasPrefix(arg, "--token-dir="):
			raw, nextIdx, err := optionValue(args, idx, "--token-dir")
			if err != nil {
				return options, err
			}
			raw = strings.TrimSpace(raw)
			if raw == "" {
				return options, fmt.Errorf("invalid token directory %q", raw)
			}
			options.TokenDir = raw
			idx = nextIdx
		case arg == "--refresh-messages" || strings.HasPrefix(arg, "--refresh-messages="):
			raw, nextIdx, err := optionValue(args, idx, "--refresh-messages")
			if err != nil {
				return options, err
			}
			duration, err := parseRefreshInterval(raw, "--refresh-messages")
			if err != nil {
				return options, err
			}
			options.RefreshMessagesInterval = duration
			idx = nextIdx
		case arg == "--refresh-tree" || strings.HasPrefix(arg, "--refresh-tree="):
			raw, nextIdx, err := optionValue(args, idx, "--refresh-tree")
			if err != nil {
				return options, err
			}
			duration, err := parseRefreshInterval(raw, "--refresh-tree")
			if err != nil {
				return options, err
			}
			options.RefreshConversationInterval = duration
			idx = nextIdx
		case arg == "--json":
			options.Command.OutputJSON = true
		case arg == "--all-day":
			options.Command.AllDay = true
		case arg == "--include-name-mentions":
			options.Command.IncludeNameMentions = true
		case arg == "--calendar" || strings.HasPrefix(arg, "--calendar="):
			raw, nextIdx, err := optionValue(args, idx, "--calendar")
			if err != nil {
				return options, err
			}
			options.Command.CalendarID = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--team" || strings.HasPrefix(arg, "--team="):
			raw, nextIdx, err := optionValue(args, idx, "--team")
			if err != nil {
				return options, err
			}
			options.Command.TeamIDs = append(options.Command.TeamIDs, strings.TrimSpace(raw))
			idx = nextIdx
		case arg == "--channel" || strings.HasPrefix(arg, "--channel="):
			raw, nextIdx, err := optionValue(args, idx, "--channel")
			if err != nil {
				return options, err
			}
			options.Command.ChannelIDs = append(options.Command.ChannelIDs, strings.TrimSpace(raw))
			idx = nextIdx
		case arg == "--chat" || strings.HasPrefix(arg, "--chat="):
			raw, nextIdx, err := optionValue(args, idx, "--chat")
			if err != nil {
				return options, err
			}
			options.Command.ChatIDs = append(options.Command.ChatIDs, strings.TrimSpace(raw))
			idx = nextIdx
		case arg == "--profile" || strings.HasPrefix(arg, "--profile="):
			raw, nextIdx, err := optionValue(args, idx, "--profile")
			if err != nil {
				return options, err
			}
			options.Command.Profile = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--since" || strings.HasPrefix(arg, "--since="):
			raw, nextIdx, err := optionValue(args, idx, "--since")
			if err != nil {
				return options, err
			}
			options.Command.Since = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--start" || strings.HasPrefix(arg, "--start="):
			raw, nextIdx, err := optionValue(args, idx, "--start")
			if err != nil {
				return options, err
			}
			options.Command.Start = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--end" || strings.HasPrefix(arg, "--end="):
			raw, nextIdx, err := optionValue(args, idx, "--end")
			if err != nil {
				return options, err
			}
			options.Command.End = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--limit" || strings.HasPrefix(arg, "--limit="):
			raw, nextIdx, err := optionValue(args, idx, "--limit")
			if err != nil {
				return options, err
			}
			limit, err := parsePositiveInt(raw)
			if err != nil {
				return options, fmt.Errorf("invalid limit value %q: expected a positive integer", raw)
			}
			options.Command.ResultLimit = limit
			idx = nextIdx
		case arg == "--query" || strings.HasPrefix(arg, "--query="):
			raw, nextIdx, err := optionValue(args, idx, "--query")
			if err != nil {
				return options, err
			}
			options.Command.Query = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--subject" || strings.HasPrefix(arg, "--subject="):
			raw, nextIdx, err := optionValue(args, idx, "--subject")
			if err != nil {
				return options, err
			}
			options.Command.Subject = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--timezone" || strings.HasPrefix(arg, "--timezone="):
			raw, nextIdx, err := optionValue(args, idx, "--timezone")
			if err != nil {
				return options, err
			}
			options.Command.Timezone = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--location" || strings.HasPrefix(arg, "--location="):
			raw, nextIdx, err := optionValue(args, idx, "--location")
			if err != nil {
				return options, err
			}
			options.Command.Location = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--body" || strings.HasPrefix(arg, "--body="):
			raw, nextIdx, err := optionValue(args, idx, "--body")
			if err != nil {
				return options, err
			}
			options.Command.Body = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--format" || strings.HasPrefix(arg, "--format="):
			raw, nextIdx, err := optionValue(args, idx, "--format")
			if err != nil {
				return options, err
			}
			options.Command.Format = strings.ToLower(strings.TrimSpace(raw))
			idx = nextIdx
		case arg == "--output" || strings.HasPrefix(arg, "--output="):
			raw, nextIdx, err := optionValue(args, idx, "--output")
			if err != nil {
				return options, err
			}
			options.Command.OutputPath = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--poll" || strings.HasPrefix(arg, "--poll="):
			raw, nextIdx, err := optionValue(args, idx, "--poll")
			if err != nil {
				return options, err
			}
			duration, err := parseRefreshInterval(raw, "--poll")
			if err != nil {
				return options, err
			}
			options.Command.PollInterval = duration
			idx = nextIdx
		case arg == "--state-file" || strings.HasPrefix(arg, "--state-file="):
			raw, nextIdx, err := optionValue(args, idx, "--state-file")
			if err != nil {
				return options, err
			}
			options.Command.StateFile = strings.TrimSpace(raw)
			idx = nextIdx
		case arg == "--config" || strings.HasPrefix(arg, "--config="):
			raw, nextIdx, err := optionValue(args, idx, "--config")
			if err != nil {
				return options, err
			}
			options.Command.ConfigPath = strings.TrimSpace(raw)
			idx = nextIdx
		default:
			return options, fmt.Errorf("unknown argument %q", arg)
		}
	}

	if err := finalizeCommandOptions(&options); err != nil {
		return options, err
	}

	return options, nil
}

func isCommandToken(arg string) bool {
	switch arg {
	case "list", "read", "export", "watch", "search", "create", "update", "delete":
		return true
	default:
		return false
	}
}

func parseCommandOptions(args []string, idx int, options *AppOptions) (int, error) {
	if options == nil {
		return idx, fmt.Errorf("options are required")
	}
	if options.CommandMode != commandModeTUI {
		return idx, fmt.Errorf("multiple commands are not supported")
	}
	arg := args[idx]
	switch arg {
	case "list":
		if idx+1 >= len(args) {
			return idx, fmt.Errorf("missing list target")
		}
		switch args[idx+1] {
		case "teams":
			options.CommandMode = commandModeListTeams
		case "calendars":
			options.CommandMode = commandModeListCalendars
		case "events":
			options.CommandMode = commandModeListEvents
		case "channels":
			options.CommandMode = commandModeListChannels
		case "mentions":
			options.CommandMode = commandModeListMentions
		default:
			return idx, fmt.Errorf("unknown list target %q", args[idx+1])
		}
		return idx + 1, nil
	case "read":
		if idx+2 >= len(args) {
			return idx, fmt.Errorf("read requires a target type and id")
		}
		switch args[idx+1] {
		case "channel":
			options.CommandMode = commandModeReadChannel
		case "chat":
			options.CommandMode = commandModeReadChat
		default:
			return idx, fmt.Errorf("unknown read target %q", args[idx+1])
		}
		options.Command.TargetID = strings.TrimSpace(args[idx+2])
		return idx + 2, nil
	case "export":
		if idx+2 >= len(args) {
			return idx, fmt.Errorf("export requires a target type and id")
		}
		switch args[idx+1] {
		case "channel":
			options.CommandMode = commandModeExportChannel
		case "chat":
			options.CommandMode = commandModeExportChat
		default:
			return idx, fmt.Errorf("unknown export target %q", args[idx+1])
		}
		options.Command.TargetID = strings.TrimSpace(args[idx+2])
		return idx + 2, nil
	case "watch":
		if idx+1 >= len(args) {
			return idx, fmt.Errorf("missing watch target")
		}
		if args[idx+1] != "mentions" {
			return idx, fmt.Errorf("unknown watch target %q", args[idx+1])
		}
		options.CommandMode = commandModeWatchMentions
		return idx + 1, nil
	case "search":
		if idx+1 >= len(args) {
			return idx, fmt.Errorf("missing search target")
		}
		if args[idx+1] != "messages" {
			return idx, fmt.Errorf("unknown search target %q", args[idx+1])
		}
		options.CommandMode = commandModeSearchMessages
		return idx + 1, nil
	case "create":
		if idx+1 >= len(args) {
			return idx, fmt.Errorf("missing create target")
		}
		if args[idx+1] != "event" {
			return idx, fmt.Errorf("unknown create target %q", args[idx+1])
		}
		options.CommandMode = commandModeCreateEvent
		return idx + 1, nil
	case "update":
		if idx+2 >= len(args) {
			return idx, fmt.Errorf("update event requires an event id")
		}
		if args[idx+1] != "event" {
			return idx, fmt.Errorf("unknown update target %q", args[idx+1])
		}
		options.CommandMode = commandModeUpdateEvent
		options.Command.EventID = strings.TrimSpace(args[idx+2])
		return idx + 2, nil
	case "delete":
		if idx+2 >= len(args) {
			return idx, fmt.Errorf("delete event requires an event id")
		}
		if args[idx+1] != "event" {
			return idx, fmt.Errorf("unknown delete target %q", args[idx+1])
		}
		options.CommandMode = commandModeDeleteEvent
		options.Command.EventID = strings.TrimSpace(args[idx+2])
		return idx + 2, nil
	default:
		return idx, fmt.Errorf("unknown command %q", arg)
	}
}

func finalizeCommandOptions(options *AppOptions) error {
	switch options.CommandMode {
	case commandModeSearchMessages:
		if strings.TrimSpace(options.Command.Query) == "" {
			return fmt.Errorf("search messages requires --query")
		}
	case commandModeListEvents:
		if strings.TrimSpace(options.Command.Start) == "" || strings.TrimSpace(options.Command.End) == "" {
			start := time.Now().UTC()
			end := start.Add(7 * 24 * time.Hour)
			options.Command.Start = start.Format(time.RFC3339)
			options.Command.End = end.Format(time.RFC3339)
		}
	case commandModeCreateEvent:
		if strings.TrimSpace(options.Command.Subject) == "" || strings.TrimSpace(options.Command.Start) == "" || strings.TrimSpace(options.Command.End) == "" {
			return fmt.Errorf("create event requires --subject, --start, and --end")
		}
	case commandModeUpdateEvent:
		if strings.TrimSpace(options.Command.EventID) == "" {
			return fmt.Errorf("update event requires an event id")
		}
		if !hasEventMutationFields(options.Command) {
			return fmt.Errorf("update event requires at least one field to update")
		}
	case commandModeDeleteEvent:
		if strings.TrimSpace(options.Command.EventID) == "" {
			return fmt.Errorf("delete event requires an event id")
		}
	}

	if options.Command.ResultLimit == 0 {
		options.Command.ResultLimit = defaultCommandLimit(options.CommandMode)
	}
	if options.Command.Format == "" {
		if options.CommandMode == commandModeExportChannel || options.CommandMode == commandModeExportChat {
			options.Command.Format = "json"
		} else {
			options.Command.Format = "text"
		}
	}

	return nil
}

func defaultCommandLimit(mode CommandMode) int {
	switch mode {
	case commandModeListEvents:
		return 100
	case commandModeListMentions:
		return 100
	case commandModeSearchMessages:
		return 50
	case commandModeWatchMentions:
		return 100
	case commandModeReadChannel, commandModeReadChat, commandModeExportChannel, commandModeExportChat:
		return defaultMessageLimit
	default:
		return defaultMessageLimit
	}
}

func optionValue(args []string, idx int, flagName string) (string, int, error) {
	arg := args[idx]
	if arg == flagName {
		if idx+1 >= len(args) {
			return "", idx, fmt.Errorf("missing value for %s", flagName)
		}
		return args[idx+1], idx + 1, nil
	}

	prefix := flagName + "="
	if strings.HasPrefix(arg, prefix) {
		return strings.TrimPrefix(arg, prefix), idx, nil
	}

	return "", idx, fmt.Errorf("missing value for %s", flagName)
}

func parseMessageLimit(raw string) (int, error) {
	limit, err := parsePositiveInt(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid msg value %q: expected a positive integer", raw)
	}

	return limit, nil
}

func parseRefreshInterval(raw, flagName string) (time.Duration, error) {
	duration, err := parseFlexibleDuration(raw)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("invalid value %q for %s: expected a positive duration", raw, flagName)
	}

	return duration, nil
}

func parseFlexibleDuration(raw string) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("duration cannot be empty")
	}

	if strings.HasSuffix(strings.ToLower(raw), "d") {
		value, err := parsePositiveInt(strings.TrimSuffix(strings.ToLower(raw), "d"))
		if err != nil {
			return 0, err
		}
		return time.Duration(value) * 24 * time.Hour, nil
	}

	if value, err := parsePositiveInt(raw); err == nil {
		return time.Duration(value) * time.Second, nil
	}

	return time.ParseDuration(raw)
}

func parsePositiveInt(raw string) (int, error) {
	var value int
	_, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &value)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("expected a positive integer")
	}

	if fmt.Sprintf("%d", value) != strings.TrimSpace(raw) {
		return 0, fmt.Errorf("expected a positive integer")
	}

	return value, nil
}

func usageText(program string) string {
	return fmt.Sprintf(`Usage:
  %s [options]
  %s doctor [options]
  %s list teams [options]
  %s list calendars [--json]
  %s list events [--calendar <id>] [--start <rfc3339>] [--end <rfc3339>] [--limit <n>] [--json]
  %s list channels [--team <id>...] [--profile <name>] [--json]
  %s list mentions [--team <id>...] [--channel <id>...] [--chat <id>...] [--profile <name>] [--since <d|rfc3339>] [--limit <n>] [--include-name-mentions] [--json]
  %s create event --subject <text> --start <rfc3339> --end <rfc3339> [--calendar <id>] [--timezone <iana>] [--location <text>] [--body <text>] [--all-day] [--json]
  %s update event <event-id> [--calendar <id>] [--subject <text>] [--start <rfc3339>] [--end <rfc3339>] [--timezone <iana>] [--location <text>] [--body <text>] [--all-day] [--json]
  %s delete event <event-id> [--calendar <id>] [--json]
  %s read channel <channel-id> [--since <d|rfc3339>] [--limit <n>] [--json]
  %s read chat <chat-id> [--since <d|rfc3339>] [--limit <n>] [--json]
  %s export channel <channel-id> [--since <d|rfc3339>] [--limit <n>] [--format json|md] [--output <path>]
  %s export chat <chat-id> [--since <d|rfc3339>] [--limit <n>] [--format json|md] [--output <path>]
  %s watch mentions [--team <id>...] [--channel <id>...] [--chat <id>...] [--profile <name>] [--since <d|rfc3339>] [--poll <d>] [--state-file <path>] [--include-name-mentions] [--json]
  %s search messages --query <text> [--team <id>...] [--channel <id>...] [--chat <id>...] [--profile <name>] [--since <d|rfc3339>] [--limit <n>] [--json]

Options:
  -h, --help                  Show this help text
      --debug                 Shortcut for --log-level debug
      --version               Show version information
      --msg <count>           Limit each conversation to the most recent N messages
      --log-level <level>     Set log level (debug, info, warn, error)
      --token-dir <dir>       Read token-teams.jwt, token-skype.jwt, and token-chatsvcagg.jwt from a custom directory
      --refresh-messages <d>  Poll interval for the selected conversation (seconds or Go duration, default %s)
      --refresh-tree <d>      Poll interval for the conversation tree (seconds or Go duration, default %s)
      --no-live               Disable background refresh polling
      --doctor                Run diagnostics instead of launching the TUI

Examples:
  %s --msg 20
  %s --token-dir ~/.config/fossteams --debug
  %s doctor --token-dir ~/.config/fossteams
  %s list teams
  %s list calendars --json
  %s create event --subject "Design review" --start 2026-06-08T09:00:00Z --end 2026-06-08T10:00:00Z
  %s read channel 19:channel-id --limit 100
  %s search messages --query incident --since 7d --json
`, program, program, program, program, program, program, program, program, program, program, program, program, program, program, program, program, defaultLiveMessageRefreshInterval, defaultLiveConversationRefreshInterval, program, program, program, program, program, program, program, program)
}

func hasEventMutationFields(command CommandOptions) bool {
	return strings.TrimSpace(command.Subject) != "" ||
		strings.TrimSpace(command.Start) != "" ||
		strings.TrimSpace(command.End) != "" ||
		strings.TrimSpace(command.Timezone) != "" ||
		strings.TrimSpace(command.Location) != "" ||
		strings.TrimSpace(command.Body) != "" ||
		command.AllDay
}
