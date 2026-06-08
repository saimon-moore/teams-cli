package main

import (
	"context"
	"strings"
	"testing"
	"time"

	teamsgraph "github.com/saimon-moore/teams-api/pkg/graph"
	"github.com/sirupsen/logrus"
)

type stubGraphCalendarService struct {
	listCalendarsFn func() ([]teamsgraph.Calendar, error)
	listEventsFn    func(opts teamsgraph.ListEventsOptions) ([]teamsgraph.Event, error)
	createEventFn   func(input teamsgraph.CreateEventInput) (*teamsgraph.Event, error)
	updateEventFn   func(calendarID, eventID string, input teamsgraph.UpdateEventInput) (*teamsgraph.Event, error)
	deleteEventFn   func(calendarID, eventID string) error
}

func (s stubGraphCalendarService) ListCalendars() ([]teamsgraph.Calendar, error) {
	return s.listCalendarsFn()
}

func (s stubGraphCalendarService) ListEvents(opts teamsgraph.ListEventsOptions) ([]teamsgraph.Event, error) {
	return s.listEventsFn(opts)
}

func (s stubGraphCalendarService) CreateEvent(input teamsgraph.CreateEventInput) (*teamsgraph.Event, error) {
	return s.createEventFn(input)
}

func (s stubGraphCalendarService) UpdateEvent(calendarID, eventID string, input teamsgraph.UpdateEventInput) (*teamsgraph.Event, error) {
	return s.updateEventFn(calendarID, eventID, input)
}

func (s stubGraphCalendarService) DeleteEvent(calendarID, eventID string) error {
	return s.deleteEventFn(calendarID, eventID)
}

func TestRunCommandListCalendarsJSON(t *testing.T) {
	originalFactory := newGraphCalendarService
	t.Cleanup(func() { newGraphCalendarService = originalFactory })

	newGraphCalendarService = func(options AppOptions) (graphCalendarService, error) {
		return stubGraphCalendarService{
			listCalendarsFn: func() ([]teamsgraph.Calendar, error) {
				return []teamsgraph.Calendar{{
					ID:                "cal-1",
					Name:              "Primary",
					CanEdit:           true,
					IsDefaultCalendar: true,
					Owner:             teamsgraph.EmailAddress{Address: "dev@example.com"},
				}}, nil
			},
		}, nil
	}

	options, err := parseAppOptions([]string{"list", "calendars", "--json"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out strings.Builder
	if err := runCommand(context.Background(), &out, options, logrus.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), `"id": "cal-1"`) {
		t.Fatalf("expected calendar id in output, got %s", out.String())
	}
}

func TestRunCommandListEventsText(t *testing.T) {
	originalFactory := newGraphCalendarService
	t.Cleanup(func() { newGraphCalendarService = originalFactory })

	start := time.Date(2026, time.June, 8, 9, 0, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	newGraphCalendarService = func(options AppOptions) (graphCalendarService, error) {
		return stubGraphCalendarService{
			listEventsFn: func(opts teamsgraph.ListEventsOptions) ([]teamsgraph.Event, error) {
				if opts.CalendarID != "cal-1" {
					t.Fatalf("expected calendar id cal-1, got %q", opts.CalendarID)
				}
				if !opts.Start.Equal(start) {
					t.Fatalf("expected start %s, got %s", start, opts.Start)
				}
				return []teamsgraph.Event{{
					ID:       "evt-1",
					Subject:  "Design review",
					Start:    teamsgraph.DateTimeTimeZone{DateTime: start.Format(time.RFC3339)},
					End:      teamsgraph.DateTimeTimeZone{DateTime: end.Format(time.RFC3339)},
					Location: teamsgraph.Location{DisplayName: "Room 1"},
				}}, nil
			},
		}, nil
	}

	options, err := parseAppOptions([]string{"list", "events", "--calendar", "cal-1", "--start", start.Format(time.RFC3339), "--end", end.Format(time.RFC3339)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out strings.Builder
	if err := runCommand(context.Background(), &out, options, logrus.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "SUBJECT") || !strings.Contains(output, "Design review") {
		t.Fatalf("expected event table output, got %s", output)
	}
}

func TestRunCommandCreateEventJSON(t *testing.T) {
	originalFactory := newGraphCalendarService
	t.Cleanup(func() { newGraphCalendarService = originalFactory })

	newGraphCalendarService = func(options AppOptions) (graphCalendarService, error) {
		return stubGraphCalendarService{
			createEventFn: func(input teamsgraph.CreateEventInput) (*teamsgraph.Event, error) {
				if input.Subject != "Design review" {
					t.Fatalf("expected subject Design review, got %q", input.Subject)
				}
				if input.TimeZone != "Europe/Zurich" {
					t.Fatalf("expected timezone Europe/Zurich, got %q", input.TimeZone)
				}
				return &teamsgraph.Event{ID: "evt-1", Subject: input.Subject}, nil
			},
		}, nil
	}

	options, err := parseAppOptions([]string{
		"create", "event",
		"--subject", "Design review",
		"--start", "2026-06-08T09:00:00Z",
		"--end", "2026-06-08T10:00:00Z",
		"--timezone", "Europe/Zurich",
		"--json",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out strings.Builder
	if err := runCommand(context.Background(), &out, options, logrus.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), `"id": "evt-1"`) {
		t.Fatalf("expected created event json, got %s", out.String())
	}
}

func TestRunCommandDeleteEventText(t *testing.T) {
	originalFactory := newGraphCalendarService
	t.Cleanup(func() { newGraphCalendarService = originalFactory })

	newGraphCalendarService = func(options AppOptions) (graphCalendarService, error) {
		return stubGraphCalendarService{
			deleteEventFn: func(calendarID, eventID string) error {
				if calendarID != "cal-1" || eventID != "evt-7" {
					t.Fatalf("unexpected delete target %q/%q", calendarID, eventID)
				}
				return nil
			},
		}, nil
	}

	options, err := parseAppOptions([]string{"delete", "event", "evt-7", "--calendar", "cal-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var out strings.Builder
	if err := runCommand(context.Background(), &out, options, logrus.New()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out.String(), "deleted event evt-7") {
		t.Fatalf("expected delete confirmation, got %s", out.String())
	}
}
