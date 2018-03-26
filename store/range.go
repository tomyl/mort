package store

import "time"

const (
	dateFormat = "Jan 02 Mon"
)

// TimeRange reprents a date range.
type TimeRange struct {
	Start time.Time
	End   time.Time
	Days  int
}

func (r *TimeRange) String() string {
	if r.Days <= 1 {
		return r.Start.Local().Format(dateFormat)
	}
	return r.Start.Local().Format(dateFormat) + " to " + r.End.Local().AddDate(0, 0, -1).Format(dateFormat)
}

// IsZero returns true if the TimeRange hasn't been initialized.
func (r *TimeRange) IsZero() bool {
	return r.Start.IsZero()
}

// Prev moves time range to previous day/week.
func (r *TimeRange) Prev() {
	r.Start = r.Start.AddDate(0, 0, -r.Days)
	r.End = r.End.AddDate(0, 0, -r.Days)
}

// Next moves time range to next day/week.
func (r *TimeRange) Next() {
	r.Start = r.Start.AddDate(0, 0, r.Days)
	r.End = r.End.AddDate(0, 0, r.Days)
}

// Today sets time range to today only.
func (r *TimeRange) Today() {
	r.Days = 1
	r.Start = beginningOfDay(time.Now()).UTC()
	r.End = r.Start.AddDate(0, 0, r.Days)
}

// Week sets time range to the week of currently selected start day.
func (r *TimeRange) Week() {
	if r.Start.IsZero() {
		r.Start = beginningOfDay(time.Now()).UTC()
	}

	r.Days = 7

	wd := r.Start.Weekday()

	if wd == 0 {
		wd = 7
	}

	r.Start = r.Start.AddDate(0, 0, -int(wd))
	r.End = r.Start.AddDate(0, 0, r.Days)
}

func beginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}
