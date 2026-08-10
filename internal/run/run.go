// Package run defines the lifecycle of one collection run and the events that
// explain what happened during it.
package run

import "time"

// Status is a position in the run state machine:
//
//	queued → running → succeeded | partial | failed | canceled | interrupted
//
// Terminal states never transition again.
type Status string

const (
	Queued      Status = "queued"
	Running     Status = "running"
	Succeeded   Status = "succeeded"
	Partial     Status = "partial"
	Failed      Status = "failed"
	Canceled    Status = "canceled"
	Interrupted Status = "interrupted"
)

// Trigger records what caused a run to start.
const TriggerManual = "manual"

// Event levels. These mirror slog levels so terminal output and stored events
// can be read together.
const (
	LevelInfo  = "info"
	LevelWarn  = "warn"
	LevelError = "error"
)

// MaxEventsPerRun bounds how much one run can write. A single malformed input
// file must not be able to flood the event table.
const MaxEventsPerRun = 1000

// DefaultEventRetention bounds how long events are kept unless configuration
// says otherwise. Runs themselves are never purged: losing events costs
// explainability, losing runs costs correctness.
const DefaultEventRetention = 30 * 24 * time.Hour

// StaleAfter is how long a run may go without a heartbeat before a later
// process treats it as interrupted. It must exceed HeartbeatInterval by enough
// margin that a live but busy run is never misjudged.
const StaleAfter = 2 * time.Minute

// HeartbeatInterval is how often a running process refreshes its heartbeat.
const HeartbeatInterval = 10 * time.Second

// IsTerminal reports whether the status can still change.
func (s Status) IsTerminal() bool {
	switch s {
	case Queued, Running:
		return false
	default:
		return true
	}
}

// Event is one entry explaining what happened during a run.
type Event struct {
	Level   string
	Message string
	Detail  map[string]any
}

// Counts summarizes what a run produced.
type Counts struct {
	Imported  int
	Duplicate int
	Failed    int
}
