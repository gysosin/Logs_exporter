package collectors

import (
	"sync"
	"time"
)

// FileEvent represents one filesystem action.
type FileEvent struct {
	Path      string    `json:"path"`
	Event     string    `json:"event"` // e.g. CREATE, WRITE, DELETE, RENAME, ACCESS
	PID       int       `json:"pid,omitempty"`
	UID       int       `json:"uid,omitempty"`
	User      string    `json:"user,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// in-memory ring buffer (maxEvents) -----------------------------------------
// ---------------------------------------------------------------------------

var (
	feMu      sync.Mutex
	feBuf     []FileEvent
	maxEvents = 10_000
)

// AddFileEvent appends an event, trimming the oldest when the ring is full.
func AddFileEvent(e FileEvent) {
	feMu.Lock()
	defer feMu.Unlock()

	feBuf = append(feBuf, e)
	if len(feBuf) > maxEvents {
		feBuf = feBuf[len(feBuf)-maxEvents:]
	}
}

// GetFileEvents returns a *copy* of the current buffer (thread-safe).
func GetFileEvents() []FileEvent {
	feMu.Lock()
	defer feMu.Unlock()

	out := make([]FileEvent, len(feBuf))
	copy(out, feBuf)
	return out
}

// ClearFileEvents erases the buffer (used after successful push).
func ClearFileEvents() {
	feMu.Lock()
	defer feMu.Unlock()
	feBuf = nil
}
