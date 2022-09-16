package controllers

type MesheryControllerStatus int

const (
	Deployed MesheryControllerStatus = iota
	Deploying
	Undeployed
	Enabled
	Running
	// we don't know since we have not checked yet
	Unknown
)

func (mcs MesheryControllerStatus) String() string {
	switch mcs {
	case Deployed:
		return "Deployed"
	case Deploying:
		return "Deploying"
	case Enabled:
		return "Enabled"
	case Running:
		return "Running"
	case Undeployed:
		return "Undeployed"
	case Unknown:
		return "Unknown"
	}
	return "unknown"
}

type IMesheryController interface {
	GetName() string
	GetStatus() MesheryControllerStatus
	Deploy(force bool) error //If force is set to false && controller is in "Undeployed", then Deployment will be skipped. Set force=true for explicit install.
	Undeploy() error
	GetPublicEndpoint() (string, error)
	GetVersion() (string, error)
}
