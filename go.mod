module github.com/layer5io/gokit

go 1.13

replace (
	github.com/kudobuilder/kuttl => github.com/layer5io/kuttl v0.4.1-0.20200806180306-b7e46afd657f
	vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
)

require (
	github.com/google/uuid v1.1.1
	github.com/kr/pretty v0.2.1 // indirect
	github.com/layer5io/learn-layer5/smi-conformance v0.0.0-20201022191033-40468652a54f
	github.com/sirupsen/logrus v1.6.0
	golang.org/x/sys v0.0.0-20200803210538-64077c9b5642 // indirect
	helm.sh/helm/v3 v3.3.1
	k8s.io/api v0.18.8
	k8s.io/apimachinery v0.18.8
	k8s.io/client-go v0.18.8
	rsc.io/letsencrypt v0.0.3 // indirect
)
