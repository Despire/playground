package tracker

// Event represents state of the client during communication
// with the tracker.
type Event string

const (
	// EventStarted must be included by the first request to the tracker.
	EventStarted Event = "started"
	// EventStopped must be included if the client Eventis shutting down gracefully.
	EventStopped Event = "stopped"
	// EventCompleted must be included when the download completes.
	EventCompleted Event = "completed"
)
