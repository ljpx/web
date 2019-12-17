package web

// Config defines a set of configuration values that dictate how the handler
// behaves at a global level.
type Config struct {
	ProblemDetailsTypePrefix string
	DebuggingEnabled         bool
	JSONContentLengthLimit   int64
}
