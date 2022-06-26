package controllers

type MesheryControllerStatus int

const (
	Deployed MesheryControllerStatus = iota
	Deploying
	NotDeployed
	// we don't know since we have not checked yet
	Unknown
)

func (mcs MesheryControllerStatus) String() string {
	switch mcs {
	case Deployed:
		return "Deployed"
	case Deploying:
		return "Deploying"
	case NotDeployed:
		return "Not Deployed"
	case Unknown:
		return "Unknown"
	}
	return "unknown"
}

type IMesheryController interface {
	GetName() string
	GetStatus() MesheryControllerStatus
	Deploy() error
	GetPublicEndpoint() (string, error)
	GetVersion() (string, error)
}
