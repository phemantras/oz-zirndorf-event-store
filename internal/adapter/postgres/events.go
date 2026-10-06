package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/phemantras/oz-zirndorf-event-store/internal/adapter/postgres/db"
	"github.com/phemantras/oz-zirndorf-event-store/internal/core"
)

// noRowsAffected is what an UPDATE or DELETE reports when no row has the
// given id.
const noRowsAffected = 0

// EventRepo implements core.EventRepo on the events and timetable_entries
// tables. Create and Update write several rows; the core calls them inside
// a transaction of TxRunner, so they are atomic there.
type EventRepo struct {
	queries *db.Queries
}

var _ core.EventRepo = (*EventRepo)(nil)

// NewEventRepo returns an event repository on conn, usually the pool.
func NewEventRepo(conn db.DBTX) *EventRepo {
	return &EventRepo{queries: db.New(conn)}
}

// List returns all events with their timetables in no particular order;
// the core sorts them. All entries are read with one query.
func (r *EventRepo) List(ctx context.Context) ([]core.Event, error) {
	rows, err := r.queries.ListEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("list events: %w", err)
	}
	entryRows, err := r.queries.ListTimetableEntries(ctx)
	if err != nil {
		return nil, fmt.Errorf("list timetable entries: %w", err)
	}
	return eventsWithTimetables(eventsFromRows(rows), entryRows), nil
}

// ListOverlapping returns the events whose stored effective period
// overlaps overlap, with their timetables, in no particular order. The
// database applies only the predicate of AD-16; entries are read for the
// found events with one query.
func (r *EventRepo) ListOverlapping(ctx context.Context, overlap core.Overlap) ([]core.Event, error) {
	params := db.ListEventsOverlappingParams{Lo: optionalInstantParam(overlap.Lo), Hi: optionalInstantParam(overlap.Hi)}
	rows, err := r.queries.ListEventsOverlapping(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list overlapping events: %w", err)
	}
	ids := make([]pgtype.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	entryRows, err := r.queries.ListTimetableEntriesOfEvents(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("list timetable entries of overlapping events: %w", err)
	}
	return eventsWithTimetables(eventsFromRows(rows), entryRows), nil
}

// eventsWithTimetables gives each of events the rows of entryRows that
// belong to it.
func eventsWithTimetables(events []core.Event, entryRows []db.TimetableEntry) []core.Event {
	timetables := make(map[string][]core.TimetableEntry)
	for _, entryRow := range entryRows {
		eventID := entryRow.EventID.String()
		timetables[eventID] = append(timetables[eventID], timetableEntryFromRow(entryRow))
	}
	for i := range events {
		events[i].Timetable = timetables[events[i].ID]
	}
	return events
}

// eventRowOfQuery is any event row type that sqlc generates for the queries
// reading events. They all have the fields of db.GetEventRow, the one row
// type eventFromRow converts.
type eventRowOfQuery interface {
	db.ListEventsRow | db.ListEventsOverlappingRow | db.FindEventsByDuplicateKeyRow
}

// eventsFromRows converts event rows without their timetable.
func eventsFromRows[Row eventRowOfQuery](rows []Row) []core.Event {
	events := make([]core.Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, eventFromRow(db.GetEventRow(row)))
	}
	return events
}

// Get returns the event with id. An id that is no UUID cannot exist, so it
// yields core.ErrNotFound like a missing row.
func (r *EventRepo) Get(ctx context.Context, id string) (core.Event, error) {
	uuid, err := parseID(eventKind, id)
	if err != nil {
		return core.Event{}, err
	}
	row, err := r.queries.GetEvent(ctx, uuid)
	if err != nil {
		return core.Event{}, translateError("get event", err)
	}
	entryRows, err := r.queries.ListTimetableEntriesOfEvent(ctx, uuid)
	if err != nil {
		return core.Event{}, fmt.Errorf("list timetable entries of event: %w", err)
	}
	event := eventFromRow(row)
	for _, entryRow := range entryRows {
		event.Timetable = append(event.Timetable, timetableEntryFromRow(entryRow))
	}
	return event, nil
}

// Create inserts event and its timetable; the database generates the
// UUIDv7 IDs. A location that no longer exists yields core.ErrConflict.
func (r *EventRepo) Create(ctx context.Context, event core.Event) (core.Event, error) {
	columns, err := eventColumnsOf(event)
	if err != nil {
		return core.Event{}, err
	}
	row, err := r.queries.CreateEvent(ctx, db.CreateEventParams(columns))
	if err != nil {
		return core.Event{}, translateError("create event", err)
	}
	return r.withTimetable(ctx, db.GetEventRow(row), event.Timetable)
}

// Update replaces the event with event.ID and its whole timetable, or
// yields core.ErrNotFound, or core.ErrConflict when its location no longer
// exists.
func (r *EventRepo) Update(ctx context.Context, event core.Event) (core.Event, error) {
	uuid, err := parseID(eventKind, event.ID)
	if err != nil {
		return core.Event{}, err
	}
	columns, err := eventColumnsOf(event)
	if err != nil {
		return core.Event{}, err
	}
	row, err := r.queries.UpdateEvent(ctx, db.UpdateEventParams{
		ID:                uuid,
		Title:             columns.Title,
		TitleKey:          columns.TitleKey,
		Type:              columns.Type,
		LocationID:        columns.LocationID,
		StartDate:         columns.StartDate,
		StartTime:         columns.StartTime,
		EndDate:           columns.EndDate,
		EndTime:           columns.EndTime,
		AllDay:            columns.AllDay,
		SourceDescription: columns.SourceDescription,
		SourceUrl:         columns.SourceUrl,
		Note:              columns.Note,
		EffectiveStart:    columns.EffectiveStart,
		EffectiveEnd:      columns.EffectiveEnd,
	})
	if err != nil {
		return core.Event{}, translateError("update event", err)
	}
	if err := r.queries.DeleteTimetableEntriesOfEvent(ctx, row.ID); err != nil {
		return core.Event{}, fmt.Errorf("delete timetable entries of event: %w", err)
	}
	return r.withTimetable(ctx, db.GetEventRow(row), event.Timetable)
}

// withTimetable inserts the entries for the stored event row and returns
// the event with the stored entries.
func (r *EventRepo) withTimetable(ctx context.Context, row db.GetEventRow, entries []core.TimetableEntry) (core.Event, error) {
	event := eventFromRow(row)
	for _, entry := range entries {
		entryRow, err := r.queries.CreateTimetableEntry(ctx, db.CreateTimetableEntryParams{
			EventID:     row.ID,
			Description: entry.Description,
			Date:        dateParam(entry.Date),
			StartTime:   timeParam(entry.StartTime),
			EndTime:     timeParam(entry.EndTime),
		})
		if err != nil {
			return core.Event{}, fmt.Errorf("create timetable entry: %w", err)
		}
		event.Timetable = append(event.Timetable, timetableEntryFromRow(entryRow))
	}
	return event, nil
}

// FindByDuplicateKey returns all events with exactly the stored title key,
// start date and location ID of key, without their timetable. The core
// hands over the location ID as stored, so an unparsable one is a
// programming error and not reported as core.ErrNotFound.
func (r *EventRepo) FindByDuplicateKey(ctx context.Context, key core.DuplicateKey) ([]core.Event, error) {
	var locationID pgtype.UUID
	if err := locationID.Scan(key.LocationID); err != nil {
		return nil, fmt.Errorf("location id %q of duplicate key: %w", key.LocationID, err)
	}
	rows, err := r.queries.FindEventsByDuplicateKey(ctx, db.FindEventsByDuplicateKeyParams{
		TitleKey:   key.TitleKey,
		StartDate:  dateParam(key.StartDate),
		LocationID: locationID,
	})
	if err != nil {
		return nil, fmt.Errorf("find events by duplicate key: %w", err)
	}
	return eventsFromRows(rows), nil
}

// Delete removes the event with id; its timetable goes with it by the
// foreign key's ON DELETE CASCADE. A missing or unparsable id yields
// core.ErrNotFound.
func (r *EventRepo) Delete(ctx context.Context, id string) error {
	uuid, err := parseID(eventKind, id)
	if err != nil {
		return err
	}
	affected, err := r.queries.DeleteEvent(ctx, uuid)
	if err != nil {
		return fmt.Errorf("delete event: %w", err)
	}
	if affected == noRowsAffected {
		return fmt.Errorf("delete event %s: %w", id, core.ErrNotFound)
	}
	return nil
}

// CountByLocation returns how many events, archived ones included, refer
// to the location with locationID. The core hands over the location ID as
// stored, so an unparsable one is a programming error and not reported as
// core.ErrNotFound.
func (r *EventRepo) CountByLocation(ctx context.Context, locationID string) (int, error) {
	var uuid pgtype.UUID
	if err := uuid.Scan(locationID); err != nil {
		return 0, fmt.Errorf("location id %q to count events of: %w", locationID, err)
	}
	count, err := r.queries.CountEventsByLocation(ctx, uuid)
	if err != nil {
		return 0, fmt.Errorf("count events of location: %w", err)
	}
	return int(count), nil
}

// UpdateDerived replaces only the derived values of the event with id, or
// yields core.ErrNotFound.
func (r *EventRepo) UpdateDerived(ctx context.Context, id string, derived core.Derived) error {
	uuid, err := parseID(eventKind, id)
	if err != nil {
		return err
	}
	affected, err := r.queries.UpdateEventDerived(ctx, db.UpdateEventDerivedParams{
		ID:             uuid,
		EffectiveStart: instantParam(derived.Period.Start),
		EffectiveEnd:   instantParam(derived.Period.End),
		TitleKey:       derived.TitleKey,
	})
	if err != nil {
		return fmt.Errorf("update derived values of event: %w", err)
	}
	if affected == noRowsAffected {
		return fmt.Errorf("update derived values of event %s: %w", id, core.ErrNotFound)
	}
	return nil
}

// MarkArchived sets the archive mark to now on every event whose effective
// end is at or before now and that has no mark yet, in one statement, and
// returns how many it marked.
func (r *EventRepo) MarkArchived(ctx context.Context, now time.Time) (int, error) {
	marked, err := r.queries.MarkEventsArchived(ctx, instantParam(now))
	if err != nil {
		return 0, fmt.Errorf("mark archived events: %w", err)
	}
	return int(marked), nil
}

// eventColumns are the stored columns of an event besides its ID; it
// converts to the parameters of CreateEvent.
type eventColumns db.CreateEventParams

// eventColumnsOf converts event into its columns. The core hands over the
// location ID as stored, so an unparsable one is a programming error and
// not reported as core.ErrNotFound.
func eventColumnsOf(event core.Event) (eventColumns, error) {
	var locationID pgtype.UUID
	if err := locationID.Scan(event.LocationID); err != nil {
		return eventColumns{}, fmt.Errorf("location id %q of event: %w", event.LocationID, err)
	}
	return eventColumns{
		Title:             event.Title,
		TitleKey:          event.TitleKey,
		Type:              string(event.Type),
		LocationID:        locationID,
		StartDate:         dateParam(event.Times.StartDate),
		StartTime:         timeParam(event.Times.StartTime),
		EndDate:           dateParam(event.Times.EndDate),
		EndTime:           timeParam(event.Times.EndTime),
		AllDay:            event.Times.AllDay,
		SourceDescription: event.Source.Description,
		SourceUrl:         optionalText(event.Source.URL),
		Note:              optionalText(event.Note),
		EffectiveStart:    instantParam(event.Period.Start),
		EffectiveEnd:      instantParam(event.Period.End),
	}, nil
}

func eventFromRow(row db.GetEventRow) core.Event {
	return core.Event{
		ID:         row.ID.String(),
		Title:      row.Title,
		TitleKey:   row.TitleKey,
		Type:       core.EventType(row.Type),
		LocationID: row.LocationID.String(),
		Times: core.EventTimes{
			StartDate: localDateOf(row.StartDate),
			StartTime: localTimeOf(row.StartTime),
			EndDate:   localDateOf(row.EndDate),
			EndTime:   localTimeOf(row.EndTime),
			AllDay:    row.AllDay,
		},
		Source:    core.EventSource{Description: row.SourceDescription, URL: row.SourceUrl.String},
		Note:      row.Note.String,
		Period:    core.Period{Start: row.EffectiveStart.Time, End: row.EffectiveEnd.Time},
		ImportKey: row.ImportKey.String,
	}
}

func timetableEntryFromRow(row db.TimetableEntry) core.TimetableEntry {
	return core.TimetableEntry{
		ID:          row.ID.String(),
		Description: row.Description,
		Date:        localDateOf(row.Date),
		StartTime:   localTimeOf(row.StartTime),
		EndTime:     localTimeOf(row.EndTime),
	}
}

// dateParam stores a missing (zero) date as NULL.
func dateParam(date core.LocalDate) pgtype.Date {
	if date.IsZero() {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: time.Date(date.Year, date.Month, date.Day, 0, 0, 0, 0, time.UTC), Valid: true}
}

func localDateOf(date pgtype.Date) core.LocalDate {
	if !date.Valid {
		return core.LocalDate{}
	}
	year, month, day := date.Time.Date()
	return core.LocalDate{Year: year, Month: month, Day: day}
}

// timeParam stores an unknown (nil) time as NULL, never as 00:00.
func timeParam(timeOfDay *core.LocalTime) pgtype.Time {
	if timeOfDay == nil {
		return pgtype.Time{}
	}
	sinceMidnight := time.Duration(timeOfDay.Hour)*time.Hour + time.Duration(timeOfDay.Minute)*time.Minute
	return pgtype.Time{Microseconds: sinceMidnight.Microseconds(), Valid: true}
}

func localTimeOf(timeOfDay pgtype.Time) *core.LocalTime {
	if !timeOfDay.Valid {
		return nil
	}
	sinceMidnight := time.Duration(timeOfDay.Microseconds) * time.Microsecond
	return &core.LocalTime{
		Hour:   int(sinceMidnight / time.Hour),
		Minute: int(sinceMidnight % time.Hour / time.Minute),
	}
}

func instantParam(instant time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: instant, Valid: true}
}

// optionalInstantParam gives an open (nil) bound as NULL.
func optionalInstantParam(instant *time.Time) pgtype.Timestamptz {
	if instant == nil {
		return pgtype.Timestamptz{}
	}
	return instantParam(*instant)
}
