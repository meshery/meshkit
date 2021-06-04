module github.com/layer5io/meshkit

go 1.13

replace (
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/kudobuilder/kuttl => github.com/layer5io/kuttl v0.4.1-0.20200806180306-b7e46afd657f
	golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
	vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
)

require (
	github.com/go-logr/logr v0.1.0
	github.com/go-sql-driver/mysql v1.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e // indirect
	github.com/google/go-cmp v0.5.2 // indirect
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.3.1 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/nats-io/nats.go v1.9.1
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/pelletier/go-toml v1.6.0 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/spf13/cobra v1.1.3
	github.com/spf13/viper v1.7.1
	go.opencensus.io v0.22.4 // indirect
	golang.org/x/net v0.0.0-20200822124328-c89045814202 // indirect
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d // indirect
	golang.org/x/sync v0.0.0-20200625203802-6e8e738ad208 // indirect
	golang.org/x/sys v0.0.0-20201009025420-dfb3f7c4e634 // indirect
	golang.org/x/text v0.3.3 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/genproto v0.0.0-20200804131852-c06518451d9c // indirect
	google.golang.org/grpc v1.31.0 // indirect
	google.golang.org/protobuf v1.25.0 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.20.10
	helm.sh/helm/v3 v3.3.1
	k8s.io/api v0.18.12
	k8s.io/apimachinery v0.18.12
	k8s.io/cli-runtime v0.18.12
	k8s.io/client-go v0.18.12
	rsc.io/letsencrypt v0.0.3 // indirect
)
