package events

type Severity string

// returns -1 for an invalid Severity
func (s Severity) GetSeverityNumber() int {
	switch s {
	case EMERGENCY:
		return 0
	case ALERT:
		return 1
	case CRITICAL:
		return 2
	case ERROR:
		return 3
	case WARNING:
		return 4
	case NOTICE:
		return 5
	case INFO:
		return 6
	case DEBUG:
		return 7
	default:
		return -1
	}

}

// Available Severity levels
const (
	EMERGENCY = "Emergency"
	ALERT     = "Alert"
	CRITICAL  = "Critical"
	ERROR     = "Error"
	WARNING   = "Warning"
	NOTICE    = "Notice"
	INFO      = "Informational"
	DEBUG     = "Debug"
)
