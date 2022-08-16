package compgen

// responsible for generating components using different sorts of manifests
type ComponentsGenerator interface {
	generate() ([]Component, error)
}

type Component struct {
	Schema     string
	Definition string
}
