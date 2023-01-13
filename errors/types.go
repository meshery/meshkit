package errors

type (
	Error struct {
		Code                 string
		Severity             Severity
		ShortDescription     []string
		LongDescription      []string
		ProbableCause        []string
		SuggestedRemediation []string
	}
)

type Severity int

const (
	Emergency = iota // System unusable
	None             // None severity
	Alert            // Immediate action needed
	Critical         // Critical condition—default level
	Fatal            // Fatal condition

)

var (
	NoneString = []string{"None"}
)
