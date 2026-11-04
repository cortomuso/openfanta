package domain

// Period says how far a real match has got. The zero value means the provider's state says
// nothing about it (postponed, cancelled, unknown states).
type Period string

// Periods, in the order a match goes through them.
const (
	PeriodNone       Period = ""
	PeriodNotStarted Period = "not_started"
	PeriodFirstHalf  Period = "first_half"
	PeriodHalfTime   Period = "half_time"
	PeriodSecondHalf Period = "second_half"
	PeriodExtraTime  Period = "extra_time" // extra time or the break before it
	PeriodPenalties  Period = "penalties"  // the shoot-out
	PeriodFinished   Period = "finished"
)

// Valid reports whether p is a known period or none.
func (p Period) Valid() bool {
	switch p {
	case PeriodNone, PeriodNotStarted, PeriodFirstHalf, PeriodHalfTime, PeriodSecondHalf, PeriodExtraTime, PeriodPenalties, PeriodFinished:
		return true
	}
	return false
}

// Score is a real match's goals as the provider reports them. Unlike the goals
// derived from events for scoring, it includes extra time and never a shoot-out.
type Score struct {
	Home, Away int
}
