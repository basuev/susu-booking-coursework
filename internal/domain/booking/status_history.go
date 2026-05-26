package booking

import "time"

type StatusHistoryEntry struct {
	OldStatus Status
	NewStatus Status
	Reason    string
	ChangedAt time.Time
}
