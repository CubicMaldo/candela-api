package domain

import "time"

// StreakState is the persisted weekly-streak counters of a user. It is the
// only input the streak rules need, which keeps the calculation pure and
// testable without a database.
type StreakState struct {
	WeeksCurrent int
	WeeksMax     int
	WeekCheckins int
	WeekStart    *time.Time
	LastCheckin  *time.Time
}

// StartOfWeek returns midnight UTC of the Monday of the week containing t.
func StartOfWeek(t time.Time) time.Time {
	t = t.UTC()
	// time.Sunday == 0; treat Monday as the first day of the week.
	daysSinceMonday := (int(t.Weekday()) + 6) % 7
	monday := t.AddDate(0, 0, -daysSinceMonday)
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}

// ApplyCheckInToStreak folds one check-in at `now` into `s` and returns the new
// state plus the points multiplier and the weekly consistency bonus it earns.
//
// Rules (functional spec §14): the multiplier grows with check-ins inside the
// current week (2+ → x1.5, 4+ → x2), and the first check-in of a new week pays
// a consistency bonus once the user has strung together 4+ (or 8+) weeks.
//
// Pure: no clock, no I/O. Callers must persist the returned state.
func ApplyCheckInToStreak(s StreakState, now time.Time) (next StreakState, multiplier float64, weeklyBonus int) {
	next = s
	weekStart := StartOfWeek(now)

	if next.WeekStart == nil || next.WeekStart.Before(weekStart) {
		lastWeekStart := weekStart.AddDate(0, 0, -7)
		hadActivityLastWeek := next.LastCheckin != nil &&
			!next.LastCheckin.Before(lastWeekStart)

		if hadActivityLastWeek {
			next.WeeksCurrent++
		} else {
			next.WeeksCurrent = 1
		}
		if next.WeeksCurrent > next.WeeksMax {
			next.WeeksMax = next.WeeksCurrent
		}
		next.WeekCheckins = 0
		next.WeekStart = &weekStart
	}

	next.WeekCheckins++
	nowCopy := now
	next.LastCheckin = &nowCopy

	switch {
	case next.WeekCheckins >= 4:
		multiplier = 2.0
	case next.WeekCheckins >= 2:
		multiplier = 1.5
	default:
		multiplier = 1.0
	}

	if next.WeekCheckins == 1 {
		switch {
		case next.WeeksCurrent >= 8:
			weeklyBonus = 10
		case next.WeeksCurrent >= 4:
			weeklyBonus = 5
		}
	}

	return next, multiplier, weeklyBonus
}
