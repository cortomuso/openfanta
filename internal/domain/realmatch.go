package domain

// Exception says a real match did not go the usual way. The zero value is a
// normal match, whether or not it has been played yet.
type Exception string

// Exceptions.
const (
	ExceptionNone      Exception = ""
	ExceptionPostponed Exception = "postponed" // moved out of its round, no date or a later one
	ExceptionSuspended Exception = "suspended" // started and stopped, to be resumed or replayed
)

// Valid reports whether e is a known exception.
func (e Exception) Valid() bool {
	return e == ExceptionNone || e == ExceptionPostponed || e == ExceptionSuspended
}
