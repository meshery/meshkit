package controllers

type MesheryControllerStatus int

const (
	Deployed    MesheryControllerStatus = iota //The controller is deployed(default behavior)
	Deploying                                  //The controller is being deployed
	NotDeployed                                //The controller is not deployed yet
	Undeployed                                 //The controller has been intentionally undeployed. This state is useful to avoid automatic redeployment.
	// we don't know since we have not checked yet
	Enabled
	Running
	Connected
	Unknown
)

const (
	MeshSync      = "meshsync"
	MesheryBroker = "meshery-broker"
	MesheryServer = "meshery-server"
)

func (mcs MesheryControllerStatus) String() string {
	switch mcs {
	case Deployed:
		return "Deployed"
	case Deploying:
		return "Deploying"
	case NotDeployed:
		return "Not Deployed"
	case Undeployed:
		return "Undeployed"
	case Enabled:
		return "Enabled"
	case Running:
		return "Running"
	case Connected:
		return "Connected"
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
	GeEndpointForPort(portName string) (string, error)
}
