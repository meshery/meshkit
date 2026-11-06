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
	// ErrorV2 addresses limitations of the Error struct defined above:
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
		// Causes is the nested form NewV3 records: each probable cause with the
		// remediations that answer it. ProbableCause and SuggestedRemediation are
		// derived from it (FlattenCauses). Nil for New and NewV2 errors, and then
		// omitted from JSON, so their serialized form is unchanged.
		Causes []Cause `json:",omitempty"`
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
