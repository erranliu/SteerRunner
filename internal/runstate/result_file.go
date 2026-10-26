package runstate

// ResultFile uses the original fixed-file preparation facts, then seals once.
// Sealed includes failed collection; it is never reset by resume.
type ResultFile struct {
	Progress
	Sealed bool `json:"sealed"`
}
