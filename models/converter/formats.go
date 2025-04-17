package converter

type DesignFormat string

const (
	HelmChart     DesignFormat = "helm-chart"
	DockerCompose DesignFormat = "Docker Compose"
	K8sManifest   DesignFormat = "Kubernetes Manifest"
	Design        DesignFormat = "Design"
)