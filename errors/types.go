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
	// Limitations of Error struct defined above:
	// There are different types of Errors. Each type of error contains different information.
	// Short and Long Descriptions accept only strings and so they cannot contain structured information.
	// e.g. The Validation Error information (instancePath, badValue etc.) cannot be contained in Error struct (or not in a structured way that the clients can make use of it)
	//
	// ErrorV2 struct adds the ability to express all different types of errors.
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
