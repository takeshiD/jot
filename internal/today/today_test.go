package today

import (
	"testing"
	"time"

	"github.com/tkcd/jot/internal/tw"
)

func at(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		panic(err)
	}
	return t
}

func twTime(t time.Time) string { return t.UTC().Format("20060102T150405Z") }

func TestDayBoundaryAndCarryOver(t *testing.T) {
	r := Default()
	pending := tw.Task{UUID: "p", Status: "pending", Today: "2026-11-24"}
	doneToday := tw.Task{UUID: "d", Status: "completed", Today: "2026-11-24", End: twTime(at("2026-11-24 18:00:00"))}
	if r.Of(pending, at("2026-11-24 23:59:59")) != Active {
		t.Fatal("pending should be Active on its day")
	}
	if r.Of(doneToday, at("2026-11-24 23:59:59")) != DoneToday {
		t.Fatal("completed today stays on Today until the day ends")
	}
	// Next day: unfinished is carried over, completed drops off.
	if r.Of(pending, at("2026-11-25 00:00:00")) != CarriedOver {
		t.Fatal("unfinished should carry over")
	}
	if r.Of(doneToday, at("2026-11-25 00:00:00")) != NotToday {
		t.Fatal("completed yesterday must leave Today")
	}
	// Many days later still carried over.
	if r.Of(pending, at("2026-12-10 09:00:00")) != CarriedOver {
		t.Fatal("carry over keeps going")
	}
	// carry_over = false: unfinished items from earlier days drop off.
	r2 := Rules{CarryOver: false}
	if r2.In(pending, at("2026-11-25 00:00:00")) {
		t.Fatal("no carry over")
	}
	// Deleted / not on Today
	if r.In(tw.Task{Status: "deleted", Today: "2026-11-24"}, at("2026-11-24 10:00:00")) {
		t.Fatal("deleted")
	}
	if r.In(tw.Task{Status: "pending"}, at("2026-11-24 10:00:00")) {
		t.Fatal("no jottoday")
	}
}

func TestDayStartHour(t *testing.T) {
	r := Rules{DayStartHour: 4, CarryOver: true}
	if d := r.Day(at("2026-11-25 03:59:59")); d != "2026-11-24" {
		t.Fatal(d)
	}
	if d := r.Day(at("2026-11-25 04:00:00")); d != "2026-11-25" {
		t.Fatal(d)
	}
	done := tw.Task{Status: "completed", Today: "2026-11-24", End: twTime(at("2026-11-25 01:30:00"))}
	if r.Of(done, at("2026-11-25 03:00:00")) != DoneToday {
		t.Fatal("01:30 completion belongs to the 24th when day starts at 4")
	}
	if r.Of(done, at("2026-11-25 04:00:01")) != NotToday {
		t.Fatal("after 4:00 it is a new day")
	}
}

func TestListOrderAndMove(t *testing.T) {
	now := at("2026-11-24 12:00:00")
	all := []tw.Task{
		{UUID: "c", Status: "pending", Today: "2026-11-24", Order: 3, HasOrder: true},
		{UUID: "a", Status: "pending", Today: "2026-11-24", Order: 1, HasOrder: true},
		{UUID: "x", Status: "pending"},
		{UUID: "n", Status: "pending", Today: "2026-11-24", Entry: "20261124T010000Z"},
		{UUID: "b", Status: "pending", Today: "2026-11-23", Order: 2, HasOrder: true},
	}
	l := Default().List(all, now)
	ids := ""
	for _, t := range l {
		ids += t.UUID
	}
	if ids != "abcn" {
		t.Fatal(ids)
	}
	if NextOrder(l) != 4 {
		t.Fatal(NextOrder(l))
	}
	ch, ni, ok := Move(l, 2, -1) // move c up
	if !ok || ni != 1 {
		t.Fatal(ok, ni)
	}
	// a=1 stays; c->2, b->3, n->4 (n had no order)
	want := map[string]int{"c": 2, "b": 3, "n": 4}
	if len(ch) != len(want) {
		t.Fatal(ch)
	}
	for _, c := range ch {
		if want[c.UUID] != c.Order {
			t.Fatal(ch)
		}
	}
	if _, _, ok := Move(l, 0, -1); ok {
		t.Fatal("cannot move first up")
	}
	if _, _, ok := Move(l, 3, 1); ok {
		t.Fatal("cannot move last down")
	}
}
