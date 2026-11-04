package runstate

import "time"

// ResultFile uses the original fixed-file preparation facts, then seals once.
// Sealed includes failed collection; it is never reset by resume.
type ResultFile struct {
	Progress
	Sealed bool `json:"sealed"`
}

// ResultFileEnd belongs to the business-result/stop transaction, independently
// of optional file preparation. Only the frozen snapshot contract uses it.
type ResultFileEnd struct {
	ID       string    `json:"id"`
	Deadline time.Time `json:"deadline"`
}
