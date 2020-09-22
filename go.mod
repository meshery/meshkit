module github.com/layer5io/gokit

go 1.13

replace github.com/kudobuilder/kuttl => github.com/layer5io/kuttl v0.4.1-0.20200806180306-b7e46afd657f

require (
	github.com/fvbommel/sortorder v1.0.1 // indirect
	github.com/google/uuid v1.1.2
	github.com/layer5io/learn-layer5/smi-conformance v0.0.0-20200916172547-79cf11334bd7
	github.com/sirupsen/logrus v1.6.0
	helm.sh/helm/v3 v3.3.3
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.19.2
	k8s.io/client-go v0.18.8
	rsc.io/letsencrypt v0.0.3 // indirect
)
