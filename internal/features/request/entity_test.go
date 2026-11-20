package request

import "testing"

func TestStatusValid(t *testing.T) {
	tests := []struct {
		status Status
		want   bool
	}{
		{OpenStatus, true},
		{InProgressStatus, true},
		{WaitingForUserStatus, true},
		{ClosedStatus, true},
		{RejectedStatus, true},
		{Status("unknown"), false},
	}
	for _, test := range tests {
		t.Run(string(test.status), func(t *testing.T) {
			if got := test.status.Valid(); got != test.want {
				t.Fatalf("Status.Valid() = %v, want %v", got, test.want)
			}
		})
	}
}
