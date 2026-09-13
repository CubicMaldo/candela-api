package domain

import (
	"testing"
	"time"
)

func at(y int, m time.Month, d, h int) time.Time {
	return time.Date(y, m, d, h, 0, 0, 0, time.UTC)
}

func ptr(t time.Time) *time.Time { return &t }

func TestStartOfWeekIsMonday(t *testing.T) {
	// 2024-06-05 is a Wednesday; 2024-06-09 the following Sunday. Both belong
	// to the week starting Monday 2024-06-03.
	want := at(2024, time.June, 3, 0)
	for _, day := range []time.Time{
		at(2024, time.June, 3, 0),  // Monday, exactly the boundary
		at(2024, time.June, 5, 14), // Wednesday
		at(2024, time.June, 9, 23), // Sunday, last hour of the week
	} {
		if got := StartOfWeek(day); !got.Equal(want) {
			t.Errorf("StartOfWeek(%s) = %s, want %s", day, got, want)
		}
	}
}

func TestApplyCheckInToStreakMultiplier(t *testing.T) {
	week := at(2024, time.June, 3, 0)

	tests := []struct {
		name          string
		weekCheckins  int
		wantMultiplie float64
	}{
		{"first of the week is unmultiplied", 0, 1.0},
		{"second earns x1.5", 1, 1.5},
		{"third stays at x1.5", 2, 1.5},
		{"fourth earns x2", 3, 2.0},
		{"beyond four stays at x2", 9, 2.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := StreakState{
				WeeksCurrent: 1,
				WeekCheckins: tt.weekCheckins,
				WeekStart:    ptr(week),
				LastCheckin:  ptr(at(2024, time.June, 4, 20)),
			}
			_, multiplier, _ := ApplyCheckInToStreak(state, at(2024, time.June, 5, 21))
			if multiplier != tt.wantMultiplie {
				t.Errorf("multiplier = %v, want %v", multiplier, tt.wantMultiplie)
			}
		})
	}
}

func TestApplyCheckInToStreakNewWeek(t *testing.T) {
	previousWeek := at(2024, time.June, 3, 0)
	now := at(2024, time.June, 11, 20) // Tuesday of the following week

	t.Run("checked in last week continues the streak", func(t *testing.T) {
		state := StreakState{
			WeeksCurrent: 3,
			WeeksMax:     3,
			WeekCheckins: 5,
			WeekStart:    ptr(previousWeek),
			LastCheckin:  ptr(at(2024, time.June, 8, 22)),
		}
		next, multiplier, _ := ApplyCheckInToStreak(state, now)

		if next.WeeksCurrent != 4 {
			t.Errorf("WeeksCurrent = %d, want 4", next.WeeksCurrent)
		}
		if next.WeeksMax != 4 {
			t.Errorf("WeeksMax = %d, want 4", next.WeeksMax)
		}
		if next.WeekCheckins != 1 {
			t.Errorf("WeekCheckins = %d, want 1 (counter resets)", next.WeekCheckins)
		}
		if multiplier != 1.0 {
			t.Errorf("multiplier = %v, want 1.0 on the first check-in of a week", multiplier)
		}
	})

	t.Run("a skipped week restarts the streak", func(t *testing.T) {
		state := StreakState{
			WeeksCurrent: 6,
			WeeksMax:     6,
			WeekCheckins: 2,
			WeekStart:    ptr(at(2024, time.May, 27, 0)),
			LastCheckin:  ptr(at(2024, time.May, 29, 20)), // two weeks before now
		}
		next, _, bonus := ApplyCheckInToStreak(state, now)

		if next.WeeksCurrent != 1 {
			t.Errorf("WeeksCurrent = %d, want 1 after a gap", next.WeeksCurrent)
		}
		if next.WeeksMax != 6 {
			t.Errorf("WeeksMax = %d, want the record 6 to survive", next.WeeksMax)
		}
		if bonus != 0 {
			t.Errorf("bonus = %d, want 0 for a restarted streak", bonus)
		}
	})
}

func TestApplyCheckInToStreakWeeklyBonus(t *testing.T) {
	previousWeek := at(2024, time.June, 3, 0)
	lastCheckin := at(2024, time.June, 8, 22)
	now := at(2024, time.June, 11, 20)

	tests := []struct {
		name         string
		weeksCurrent int
		wantBonus    int
	}{
		{"under four weeks pays nothing", 2, 0},
		{"four weeks pays 5", 3, 5},
		{"eight weeks pays 10", 7, 10},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := StreakState{
				WeeksCurrent: tt.weeksCurrent,
				WeekCheckins: 1,
				WeekStart:    ptr(previousWeek),
				LastCheckin:  ptr(lastCheckin),
			}
			_, _, bonus := ApplyCheckInToStreak(state, now)
			if bonus != tt.wantBonus {
				t.Errorf("bonus = %d, want %d", bonus, tt.wantBonus)
			}
		})
	}

	t.Run("only the first check-in of the week pays the bonus", func(t *testing.T) {
		state := StreakState{
			WeeksCurrent: 9,
			WeekCheckins: 1,
			// Already checked in during the current week.
			WeekStart:   ptr(StartOfWeek(now)),
			LastCheckin: ptr(now.Add(-2 * time.Hour)),
		}
		_, _, bonus := ApplyCheckInToStreak(state, now)
		if bonus != 0 {
			t.Errorf("bonus = %d, want 0 on a repeat check-in", bonus)
		}
	})
}

func TestApplyCheckInToStreakFromZero(t *testing.T) {
	next, multiplier, bonus := ApplyCheckInToStreak(StreakState{}, at(2024, time.June, 5, 21))

	if next.WeeksCurrent != 1 || next.WeeksMax != 1 {
		t.Errorf("first ever check-in: got current=%d max=%d, want 1/1", next.WeeksCurrent, next.WeeksMax)
	}
	if next.WeekCheckins != 1 {
		t.Errorf("WeekCheckins = %d, want 1", next.WeekCheckins)
	}
	if multiplier != 1.0 || bonus != 0 {
		t.Errorf("got multiplier=%v bonus=%d, want 1.0/0", multiplier, bonus)
	}
}

func TestApplyCheckInToStreakDoesNotMutateInput(t *testing.T) {
	week := at(2024, time.June, 3, 0)
	original := StreakState{
		WeeksCurrent: 2,
		WeeksMax:     2,
		WeekCheckins: 1,
		WeekStart:    ptr(week),
		LastCheckin:  ptr(at(2024, time.June, 4, 20)),
	}
	before := *original.LastCheckin

	ApplyCheckInToStreak(original, at(2024, time.June, 5, 21))

	if original.WeekCheckins != 1 {
		t.Errorf("input WeekCheckins mutated to %d", original.WeekCheckins)
	}
	if !original.LastCheckin.Equal(before) {
		t.Error("input LastCheckin was mutated through the shared pointer")
	}
}
