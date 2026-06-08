package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dgrijalva/jwt-go"
	teamsapi "github.com/saimon-moore/teams-api/pkg"
	teamsgraph "github.com/saimon-moore/teams-api/pkg/graph"
	"github.com/sirupsen/logrus"
)

type graphCalendarService interface {
	ListCalendars() ([]teamsgraph.Calendar, error)
	ListEvents(opts teamsgraph.ListEventsOptions) ([]teamsgraph.Event, error)
	CreateEvent(input teamsgraph.CreateEventInput) (*teamsgraph.Event, error)
	UpdateEvent(calendarID, eventID string, input teamsgraph.UpdateEventInput) (*teamsgraph.Event, error)
	DeleteEvent(calendarID, eventID string) error
}

var newGraphCalendarService = func(options AppOptions) (graphCalendarService, error) {
	token, _, err := resolveAndValidateToken(tokenTypeGraph, options.TokenDir)
	if err != nil {
		return nil, err
	}

	jwtToken, _ := jwt.Parse(token.Value, nil)
	return teamsgraph.NewCalendarClient(http.DefaultClient, &teamsapi.TeamsToken{
		Inner: jwtToken,
		Type:  teamsapi.TokenBearer,
	}), nil
}

func commandUsesGraphBootstrap(mode CommandMode) bool {
	switch mode {
	case commandModeListCalendars, commandModeListEvents, commandModeCreateEvent, commandModeUpdateEvent, commandModeDeleteEvent:
		return true
	default:
		return false
	}
}

func commandUsesTeamsBootstrap(mode CommandMode) bool {
	if mode == commandModeTUI {
		return true
	}
	return !commandUsesGraphBootstrap(mode)
}

func runCalendarCommand(ctx context.Context, out io.Writer, options AppOptions, logger *logrus.Logger) error {
	_ = ctx

	service, err := newGraphCalendarService(options)
	if err != nil {
		return err
	}

	switch options.CommandMode {
	case commandModeListCalendars:
		calendars, err := service.ListCalendars()
		if err != nil {
			return err
		}
		if options.Command.OutputJSON {
			return writeJSON(out, calendars)
		}
		return formatCalendarsTable(out, calendars)
	case commandModeListEvents:
		listOptions, err := calendarListEventsOptions(options.Command)
		if err != nil {
			return err
		}
		events, err := service.ListEvents(listOptions)
		if err != nil {
			return err
		}
		if options.Command.OutputJSON {
			return writeJSON(out, events)
		}
		return formatEventsTable(out, events)
	case commandModeCreateEvent:
		input, err := calendarCreateEventInput(options.Command)
		if err != nil {
			return err
		}
		event, err := service.CreateEvent(input)
		if err != nil {
			return err
		}
		if options.Command.OutputJSON {
			return writeJSON(out, event)
		}
		return formatSingleEvent(out, "created", event)
	case commandModeUpdateEvent:
		input, err := calendarUpdateEventInput(options.Command)
		if err != nil {
			return err
		}
		event, err := service.UpdateEvent(options.Command.CalendarID, options.Command.EventID, input)
		if err != nil {
			return err
		}
		if options.Command.OutputJSON {
			return writeJSON(out, event)
		}
		return formatSingleEvent(out, "updated", event)
	case commandModeDeleteEvent:
		if err := service.DeleteEvent(options.Command.CalendarID, options.Command.EventID); err != nil {
			return err
		}
		if options.Command.OutputJSON {
			return nil
		}
		_, err := fmt.Fprintf(out, "deleted event %s\n", options.Command.EventID)
		return err
	default:
		logger.WithField("command_mode", options.CommandMode).Warn("unexpected calendar command mode")
		return fmt.Errorf("unsupported calendar command mode %q", options.CommandMode)
	}
}

func calendarListEventsOptions(command CommandOptions) (teamsgraph.ListEventsOptions, error) {
	start, err := time.Parse(time.RFC3339, strings.TrimSpace(command.Start))
	if err != nil {
		return teamsgraph.ListEventsOptions{}, fmt.Errorf("invalid --start value %q", command.Start)
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(command.End))
	if err != nil {
		return teamsgraph.ListEventsOptions{}, fmt.Errorf("invalid --end value %q", command.End)
	}

	return teamsgraph.ListEventsOptions{
		CalendarID: strings.TrimSpace(command.CalendarID),
		Start:      start,
		End:        end,
		TimeZone:   strings.TrimSpace(command.Timezone),
		Limit:      command.ResultLimit,
	}, nil
}

func calendarCreateEventInput(command CommandOptions) (teamsgraph.CreateEventInput, error) {
	start, err := time.Parse(time.RFC3339, strings.TrimSpace(command.Start))
	if err != nil {
		return teamsgraph.CreateEventInput{}, fmt.Errorf("invalid --start value %q", command.Start)
	}
	end, err := time.Parse(time.RFC3339, strings.TrimSpace(command.End))
	if err != nil {
		return teamsgraph.CreateEventInput{}, fmt.Errorf("invalid --end value %q", command.End)
	}

	return teamsgraph.CreateEventInput{
		CalendarID: strings.TrimSpace(command.CalendarID),
		Subject:    strings.TrimSpace(command.Subject),
		Start:      start,
		End:        end,
		TimeZone:   strings.TrimSpace(command.Timezone),
		Location:   strings.TrimSpace(command.Location),
		Body:       strings.TrimSpace(command.Body),
		AllDay:     command.AllDay,
	}, nil
}

func calendarUpdateEventInput(command CommandOptions) (teamsgraph.UpdateEventInput, error) {
	input := teamsgraph.UpdateEventInput{}

	if value := strings.TrimSpace(command.Subject); value != "" {
		input.Subject = &value
	}
	if value := strings.TrimSpace(command.Start); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return teamsgraph.UpdateEventInput{}, fmt.Errorf("invalid --start value %q", command.Start)
		}
		input.Start = &parsed
	}
	if value := strings.TrimSpace(command.End); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return teamsgraph.UpdateEventInput{}, fmt.Errorf("invalid --end value %q", command.End)
		}
		input.End = &parsed
	}
	if value := strings.TrimSpace(command.Timezone); value != "" {
		input.TimeZone = &value
	}
	if value := strings.TrimSpace(command.Location); value != "" {
		input.Location = &value
	}
	if value := strings.TrimSpace(command.Body); value != "" {
		input.Body = &value
	}
	if command.AllDay {
		allDay := true
		input.AllDay = &allDay
	}

	return input, nil
}

func formatCalendarsTable(out io.Writer, calendars []teamsgraph.Calendar) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "NAME\tID\tOWNER\tCAN_EDIT\tDEFAULT"); err != nil {
		return err
	}
	for _, calendar := range calendars {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%t\t%t\n",
			calendar.Name,
			calendar.ID,
			calendar.Owner.Address,
			calendar.CanEdit,
			calendar.IsDefaultCalendar,
		); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func formatEventsTable(out io.Writer, events []teamsgraph.Event) error {
	writer := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "ID\tSUBJECT\tSTART\tEND\tALL_DAY\tLOCATION"); err != nil {
		return err
	}
	for _, event := range events {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%t\t%s\n",
			event.ID,
			event.Subject,
			event.Start.DateTime,
			event.End.DateTime,
			event.IsAllDay,
			event.Location.DisplayName,
		); err != nil {
			return err
		}
	}
	return writer.Flush()
}

func formatSingleEvent(out io.Writer, action string, event *teamsgraph.Event) error {
	if event == nil {
		return fmt.Errorf("%s event result was empty", action)
	}
	_, err := fmt.Fprintf(out, "%s event %s %s\n", action, event.ID, event.Subject)
	return err
}
