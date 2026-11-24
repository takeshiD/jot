// Package today implements Jot's "Today" rules (membership, day boundary,
// carry-over and manual ordering). It is pure logic: the clock is a
// parameter so the date boundary is testable.
package today

import (
	"math"
	"sort"
	"time"

	"github.com/tkcd/jot/internal/tw"
)

// DateLayout is the format stored in the jottoday UDA.
const DateLayout = "2006-01-02"

// Rules configures the day boundary and carry-over behaviour.
type Rules struct {
	// DayStartHour: the "day" changes at this local hour (0-23). With 4, the
	// time 02:30 on Nov 25 still counts as Nov 24.
	DayStartHour int
	// CarryOver: unfinished Today items from earlier days stay on Today.
	CarryOver bool
}

// Default rules: day changes at midnight, unfinished items are carried over.
func Default() Rules { return Rules{DayStartHour: 0, CarryOver: true} }

// Day returns the logical local date for t.
func (r Rules) Day(t time.Time) string {
	return t.Local().Add(-time.Duration(r.DayStartHour) * time.Hour).Format(DateLayout)
}

// State describes how a task relates to Today at a given moment.
type State int

const (
	NotToday    State = iota
	Active            // on Today, pending, added today (or later)
	CarriedOver       // on Today, pending, added on an earlier day
	DoneToday         // on Today, completed during the current day
)

// Of returns the Today state of t at now.
func (r Rules) Of(t tw.Task, now time.Time) State {
	if t.Today == "" {
		return NotToday
	}
	day := r.Day(now)
	switch {
	case t.IsPending():
		if t.Today < day {
			if r.CarryOver {
				return CarriedOver
			}
			return NotToday
		}
		return Active
	case t.IsCompleted():
		end, ok := tw.ParseTime(t.End)
		if ok && r.Day(end) == day {
			return DoneToday
		}
		return NotToday
	}
	return NotToday
}

// In reports whether t is shown on Today at now.
func (r Rules) In(t tw.Task, now time.Time) bool { return r.Of(t, now) != NotToday }

// List returns the Today items in manual order: tasks with jotorder ascending,
// then tasks without jotorder (e.g. set from the CLI) by entry time.
func (r Rules) List(all []tw.Task, now time.Time) []tw.Task {
	var out []tw.Task
	for _, t := range all {
		if r.In(t, now) {
			out = append(out, t)
		}
	}
	Sort(out)
	return out
}

// Sort orders tasks by jotorder (missing last), then entry, then uuid.
func Sort(ts []tw.Task) {
	sort.SliceStable(ts, func(i, j int) bool {
		a, b := ts[i], ts[j]
		if a.HasOrder != b.HasOrder {
			return a.HasOrder
		}
		if a.HasOrder && a.Order != b.Order {
			return a.Order < b.Order
		}
		if a.Entry != b.Entry {
			return a.Entry < b.Entry
		}
		return a.UUID < b.UUID
	})
}

// NextOrder returns an order value placing a new item after all of list.
func NextOrder(list []tw.Task) int {
	max := 0.0
	for _, t := range list {
		if t.HasOrder && t.Order > max {
			max = t.Order
		}
	}
	return int(math.Floor(max)) + 1
}

// Change is an order value to write for a task.
type Change struct {
	UUID  string
	Order int
}

// Move moves the item at index i by delta (-1 up, +1 down) within list and
// returns the minimal set of jotorder writes so that the list order becomes
// 1..n. ok is false if the move is impossible.
func Move(list []tw.Task, i, delta int) (changes []Change, newIndex int, ok bool) {
	j := i + delta
	if i < 0 || i >= len(list) || j < 0 || j >= len(list) {
		return nil, i, false
	}
	ordered := append([]tw.Task(nil), list...)
	ordered[i], ordered[j] = ordered[j], ordered[i]
	return Renumber(ordered), j, true
}

// Renumber returns writes needed so that ordered[k] has jotorder k+1.
func Renumber(ordered []tw.Task) []Change {
	var ch []Change
	for k, t := range ordered {
		want := k + 1
		if !t.HasOrder || t.Order != float64(want) {
			ch = append(ch, Change{UUID: t.UUID, Order: want})
		}
	}
	return ch
}
