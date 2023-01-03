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
	// There are different types of Errors. Each type of error contains different information.
	// ErrorV2 struct adds the ability to express all different types of errors.
	// e.g. The information that a Validation Error (instancePath, badValue etc.) has cannot be contained in Error struct (or atleast in a me)
	// ErrorV2 is backwards compatible with Error struct
	ErrorV2 struct {
		Code                 string
		Severity             Severity
		ShortDescription     []string
		LongDescription      []string
		ProbableCause        []string
		SuggestedRemediation []string
		AdditionalInfo       interface{}
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
