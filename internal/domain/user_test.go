package domain

import (
	"testing"
	"time"
)

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestCalculateAge(t *testing.T) {
	tests := []struct {
		name      string
		birthDate time.Time
		now       time.Time
		want      int
	}{
		{
			name:      "birthday already passed this year",
			birthDate: date(2000, 3, 15),
			now:       date(2024, 6, 1),
			want:      24,
		},
		{
			name:      "birthday not yet this year",
			birthDate: date(2000, 12, 1),
			now:       date(2024, 6, 1),
			want:      23,
		},
		{
			name:      "exactly on birthday",
			birthDate: date(2000, 6, 1),
			now:       date(2024, 6, 1),
			want:      24,
		},
		{
			name:      "one day before birthday",
			birthDate: date(2000, 6, 2),
			now:       date(2024, 6, 1),
			want:      23,
		},
		{
			name:      "leap day birthday on non-leap year",
			birthDate: date(2000, 2, 29),
			now:       date(2023, 3, 1),
			want:      23,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{BirthDate: tt.birthDate}
			got := u.CalculateAge(tt.now)
			if got != tt.want {
				t.Errorf("CalculateAge() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestIsProtectedMode(t *testing.T) {
	now := date(2024, 6, 1)

	tests := []struct {
		name        string
		birthDate   time.Time
		isProtected bool
		want        bool
	}{
		{
			name:        "adult, flag false → not protected",
			birthDate:   date(2000, 1, 1),
			isProtected: false,
			want:        false,
		},
		{
			name:        "adult, flag true → protected",
			birthDate:   date(2000, 1, 1),
			isProtected: true,
			want:        true,
		},
		{
			name:        "minor (17), flag false → protected by age",
			birthDate:   date(2007, 6, 2),
			isProtected: false,
			want:        true,
		},
		{
			name:        "turns 18 today → not protected",
			birthDate:   date(2006, 6, 1),
			isProtected: false,
			want:        false,
		},
		{
			name:        "turns 18 tomorrow → still protected",
			birthDate:   date(2006, 6, 2),
			isProtected: false,
			want:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &User{BirthDate: tt.birthDate, IsProtected: tt.isProtected}
			got := u.IsProtectedMode(now)
			if got != tt.want {
				t.Errorf("IsProtectedMode() = %v, want %v", got, tt.want)
			}
		})
	}
}
