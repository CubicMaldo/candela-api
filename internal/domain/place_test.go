package domain

import (
	"errors"
	"testing"
)

func TestValidateGeofence(t *testing.T) {
	tests := []struct {
		name        string
		meters      int
		isBusiness  bool
		category    string
		expectedErr error
	}{
		// Global boundary checks
		{"under min global (15m)", 15, false, "BAR", ErrGeofenceOutOfRange},
		{"exact min global (20m)", 20, false, "BAR", nil},
		{"over max global (151m)", 151, true, "EVENT", ErrGeofenceOutOfRange},
		{"negative (-50m)", -50, false, "BAR", ErrGeofenceOutOfRange},
		{"zero", 0, false, "BAR", ErrGeofenceOutOfRange},

		// Regular User organizer (max 80m)
		{"user within limit (50m)", 50, false, "CAFE", nil},
		{"user exact limit (80m)", 80, false, "CAFE", nil},
		{"user exceeds limit (100m)", 100, false, "CAFE", ErrGeofenceExceedsLimit},
		{"user exceeds limit (120m)", 120, false, "CAFE", ErrGeofenceExceedsLimit},

		// Business organizer (max 120m)
		{"business within limit (80m)", 80, true, "BAR", nil},
		{"business exact limit (120m)", 120, true, "NIGHTCLUB", nil},
		{"business exceeds limit (130m)", 130, true, "NIGHTCLUB", ErrGeofenceExceedsLimit},

		// Event category (max 150m)
		{"event exact max limit (150m)", 150, true, "EVENT", nil},
		{"event non-business exact max limit (150m)", 150, false, "EVENT", nil},
		{"event within limit (100m)", 100, false, "EVENT", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateGeofence(tt.meters, tt.isBusiness, tt.category)
			if !errors.Is(err, tt.expectedErr) {
				t.Errorf("ValidateGeofence(%d, %v, %s) = %v, expected %v",
					tt.meters, tt.isBusiness, tt.category, err, tt.expectedErr)
			}
		})
	}
}
