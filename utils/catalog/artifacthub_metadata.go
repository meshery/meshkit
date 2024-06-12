package catalog

type ContainersImage struct {
	Name        string `yaml:"name,omitempty"`
	Image       string `yaml:"image,omitempty"`
	Whitelisted string `yaml:"whitelisted,omitempty"`
}

type Link struct {
	Name string `yaml:"name,omitempty"`
	URL  string `yaml:"url,omitempty"`
}

type Change struct {
	Kind        string `yaml:"kind,omitempty"`
	Description string `yaml:"description,omitempty"`
	Links       []Link `yaml:"links,omitempty"`
}

type Maintainer struct {
	Name  string `yaml:"name,omitempty"`
	Email string `yaml:"email,omitempty"`
}

type Provider struct {
	Name string `yaml:"name,omitempty"`
}

type Recommendation struct {
	URL string `yaml:"url,omitempty"`
}

type Screenshot struct {
	Title string `yaml:"title,omitempty"`
	URL   string `yaml:"url,omitempty"`
}

type ArtifactHubMetadata struct {
	Version                 string            `yaml:"version,omitempty"`
	Name                    string            `yaml:"name,omitempty"`
	DisplayName             string            `yaml:"displayName,omitempty"`
	CreatedAt               string            `yaml:"createdAt,omitempty"`
	Description             string            `yaml:"description,omitempty"`
	LogoPath                string            `yaml:"logoPath,omitempty"`
	LogoURL                 string            `yaml:"logoURL,omitempty"`
	Digest                  string            `yaml:"digest,omitempty"`
	License                 string            `yaml:"license,omitempty"`
	HomeURL                 string            `yaml:"homeURL,omitempty"`
	AppVersion              string            `yaml:"appVersion,omitempty"`
	ContainersImages        []ContainersImage `yaml:"containersImages,omitempty"`
	ContainsSecurityUpdates string            `yaml:"containsSecurityUpdates,omitempty"`
	Operator                string            `yaml:"operator,omitempty"`
	Deprecated              string            `yaml:"deprecated,omitempty"`
	Prerelease              string            `yaml:"prerelease,omitempty"`
	Keywords                []string          `yaml:"keywords,omitempty"`
	Links                   []Link            `yaml:"links,omitempty"`
	Readme                  string            `yaml:"readme,omitempty"`
	Install                 string            `yaml:"install,omitempty"`
	Changes                 []Change          `yaml:"changes,omitempty"`
	Maintainers             []Maintainer      `yaml:"maintainers,omitempty"`
	Provider                Provider          `yaml:"provider,omitempty"`
	Ignore                  []string          `yaml:"ignore,omitempty"`
	Recommendations         []Recommendation  `yaml:"recommendations,omitempty"`
	Screenshots             []Screenshot      `yaml:"screenshots,omitempty"`
	Annotations             struct {
		Key1 string `yaml:"key1,omitempty"`
		Key2 string `yaml:"key2,omitempty"`
	} `yaml:"annotations,omitempty"`
}