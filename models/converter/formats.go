package converter

type DesignFormat string

const (
	HelmChart     DesignFormat = "Helm Chart"
	DockerCompose DesignFormat = "Docker Compose"
	K8sManifest   DesignFormat = "Kubernetes Manifest"
	Design        DesignFormat = "Design"
)