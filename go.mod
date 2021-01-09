module github.com/layer5io/meshkit

go 1.13

replace (
	github.com/kudobuilder/kuttl => github.com/layer5io/kuttl v0.4.1-0.20200806180306-b7e46afd657f
	vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
)

require (
	github.com/go-kit/kit v0.10.0
	github.com/go-logr/logr v0.1.0
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.1
	github.com/kr/pretty v0.2.1 // indirect
	github.com/layer5io/learn-layer5/smi-conformance v0.0.0-20201022191033-40468652a54f
	golang.org/x/sys v0.0.0-20200803210538-64077c9b5642 // indirect
	google.golang.org/protobuf v1.23.0
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.20.10
	helm.sh/helm/v3 v3.3.1
	k8s.io/api v0.18.12
	k8s.io/apimachinery v0.18.12
	k8s.io/cli-runtime v0.18.12
	k8s.io/client-go v0.18.12
	rsc.io/letsencrypt v0.0.3 // indirect
)
