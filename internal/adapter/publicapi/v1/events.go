package v1

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// EventLister is the core query behind /v1/events (AD-7).
type EventLister interface {
	ListActiveEvents(ctx context.Context, clock core.Clock, filter core.EventFilter) ([]core.ListedEvent, error)
}

// archivePath is where past events are listed; the detail of a to before
// today points there.
const archivePath = basePath + "/archive/events"

// typeCodeSeparator joins the event type codes in a detail.
const typeCodeSeparator = ", "

// English details of the filter problems the core reports, keyed by
// parameter and problem.
var filterProblemDetails = map[core.FieldError]string{
	{Field: core.FilterFieldFrom, Problem: core.ProblemInvalidFormat}: fmt.Sprintf(detailInvalidBoundFormat, core.FilterFieldFrom),
	{Field: core.FilterFieldTo, Problem: core.ProblemInvalidFormat}:   fmt.Sprintf(detailInvalidBoundFormat, core.FilterFieldTo),
	{Field: core.FilterFieldType, Problem: core.ProblemMissing}:       detailMissingType,
	{Field: core.FilterFieldType, Problem: core.ProblemUnknownCode}:   fmt.Sprintf(detailUnknownTypeFormat, eventTypeCodes()),
	{Field: core.FilterFieldTo, Problem: core.ProblemEmptyPeriod}:     detailEmptyPeriod,
	{Field: core.FilterFieldTo, Problem: core.ProblemBeforeToday}:     fmt.Sprintf(detailBeforeTodayFormat, archivePath),
}

const (
	detailInvalidBoundFormat = "Parameter %s must be a date YYYY-MM-DD or an instant YYYY-MM-DDTHH:MM with Z or an offset such as +01:00; send the + URL-encoded as %%2B."
	detailMissingType        = "Parameter type must not be empty; it takes an event type code such as market."
	detailUnknownTypeFormat  = "Parameter type must be an event type code: %s."
	detailEmptyPeriod        = "Parameters from and to give an empty period; to must not lie before from."
	detailBeforeTodayFormat  = "Parameter to lies before today, but /v1/events lists only active events; use %s for past events."
	detailInvalidFieldFormat = "Parameter %s is invalid."
	// detailSeparator joins the details of several problems.
	detailSeparator = " "
)

func eventTypeCodes() string {
	var codes []string
	for _, eventType := range core.EventTypes() {
		codes = append(codes, string(eventType))
	}
	return strings.Join(codes, typeCodeSeparator)
}

// ListEvents returns the active events of the filter period from the core;
// an invalid filter is a 400 naming the parameter.
func (s server) ListEvents(ctx context.Context, request ListEventsRequestObject) (ListEventsResponseObject, error) {
	listed, err := s.events.ListActiveEvents(ctx, s.clock, eventFilterOf(request.Params))
	var validation *core.ValidationError
	if errors.As(err, &validation) {
		problem := problemOf(http.StatusBadRequest, filterProblemDetail(validation))
		return ListEvents400ApplicationProblemPlusJSONResponse{BadRequestApplicationProblemPlusJSONResponse(problem)}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list active events: %w", err)
	}
	events := make([]Event, 0, len(listed))
	for _, event := range listed {
		events = append(events, eventOf(event))
	}
	return ListEvents200JSONResponse{Data: events}, nil
}

func eventFilterOf(params ListEventsParams) core.EventFilter {
	filter := core.EventFilter{From: params.From, To: params.To}
	if params.Type != nil {
		for _, eventType := range *params.Type {
			filter.Types = append(filter.Types, string(eventType))
		}
	}
	return filter
}

// filterProblemDetail explains every rejected parameter in English.
func filterProblemDetail(validation *core.ValidationError) string {
	details := make([]string, 0, len(validation.Fields))
	for _, field := range validation.Fields {
		detail, ok := filterProblemDetails[field]
		if !ok {
			detail = fmt.Sprintf(detailInvalidFieldFormat, field.Field)
		}
		details = append(details, detail)
	}
	return strings.Join(details, detailSeparator)
}

// eventOf maps a listed event to the read form; it derives nothing.
func eventOf(event core.ListedEvent) Event {
	return Event{
		Title:          event.Title,
		Type:           EventType(event.Type),
		Location:       eventLocationOf(event.Location),
		StartDate:      dateOf(event.Times.StartDate),
		StartTime:      timeOf(event.Times.StartTime),
		EndDate:        optionalDateOf(event.Times.EndDate),
		EndTime:        timeOf(event.Times.EndTime),
		AllDay:         event.Times.AllDay,
		StartPrecision: TimePrecision(event.StartPrecision),
		EndPrecision:   optionalPrecisionOf(event.EndPrecision),
		Source:         Source{Description: event.Source.Description, Url: optionalText(event.Source.URL)},
		Note:           optionalText(event.Note),
		Timetable:      timetableOf(event.Timetable),
		EffectiveStart: event.Period.Start,
		EffectiveEnd:   event.Period.End,
		Archived:       event.Archived,
	}
}

func eventLocationOf(location core.Location) EventLocation {
	return EventLocation{
		Name: location.Name,
		Address: Address{
			Street:     optionalText(location.Street),
			PostalCode: optionalText(location.PostalCode),
			City:       optionalText(location.City),
		},
		Latitude:  location.Latitude,
		Longitude: location.Longitude,
		Precision: LocationPrecision(location.Precision),
		Note:      optionalText(location.Note),
	}
}

// timetableOf maps the entries in the order of the core; no timetable is
// an empty list, never null.
func timetableOf(entries []core.TimetableEntry) []TimetableEntry {
	timetable := make([]TimetableEntry, 0, len(entries))
	for _, entry := range entries {
		timetable = append(timetable, TimetableEntry{
			Description: entry.Description,
			Date:        dateOf(entry.Date),
			StartTime:   timeOf(entry.StartTime),
			EndTime:     timeOf(entry.EndTime),
		})
	}
	return timetable
}

// dateOf carries a local date in the generated date type, which writes
// YYYY-MM-DD.
func dateOf(date core.LocalDate) openapi_types.Date {
	return openapi_types.Date{Time: time.Date(date.Year, date.Month, date.Day, 0, 0, 0, 0, time.UTC)}
}

// optionalDateOf gives a missing (zero) date as null.
func optionalDateOf(date core.LocalDate) *openapi_types.Date {
	if date.IsZero() {
		return nil
	}
	value := dateOf(date)
	return &value
}

// timeOf gives an unknown (nil) time as null, a known one as HH:MM.
func timeOf(timeOfDay *core.LocalTime) *string {
	if timeOfDay == nil {
		return nil
	}
	text := timeOfDay.String()
	return &text
}

// optionalPrecisionOf gives the precision of a missing end as null.
func optionalPrecisionOf(precision core.TimePrecision) *TimePrecision {
	if precision == "" {
		return nil
	}
	value := TimePrecision(precision)
	return &value
}

// optionalText gives an empty text as null.
func optionalText(text string) *string {
	if text == "" {
		return nil
	}
	return &text
}
