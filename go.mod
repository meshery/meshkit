module github.com/layer5io/meshkit

go 1.13

replace (
	vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
)

require (
	github.com/go-kit/kit v0.10.0
	github.com/go-logr/logr v0.1.0
	github.com/google/uuid v1.1.1
	github.com/kr/pretty v0.2.1 // indirect
	github.com/layer5io/service-mesh-performance v0.3.2
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.20.10
	helm.sh/helm/v3 v3.3.1
	k8s.io/api v0.18.12
	k8s.io/apimachinery v0.18.12
	k8s.io/cli-runtime v0.18.12
	k8s.io/client-go v0.18.12
	rsc.io/letsencrypt v0.0.3 // indirect
)
