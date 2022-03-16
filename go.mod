module github.com/layer5io/meshkit

go 1.13

replace (
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/kudobuilder/kuttl => github.com/layer5io/kuttl v0.4.1-0.20200806180306-b7e46afd657f
	github.com/spf13/afero => github.com/spf13/afero v1.5.1 // Until viper bug is resolved #1161
	vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
)

require (
	github.com/evanphx/json-patch v4.12.0+incompatible // indirect
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-logr/logr v0.4.0
	github.com/google/uuid v1.2.0
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/nats-io/nats-server/v2 v2.7.4 // indirect
	github.com/nats-io/nats.go v1.13.1-0.20220308171302-2f2f6968e98d
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
	golang.org/x/net v0.0.0-20211209124913-491a49abca63 // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20210831024726-fe130286e0e2 // indirect
	gopkg.in/yaml.v2 v2.4.0
	gorm.io/driver/sqlite v1.1.4
	gorm.io/gorm v1.20.10
	helm.sh/helm/v3 v3.7.2
	k8s.io/api v0.23.0-alpha.1
	k8s.io/apimachinery v0.23.0-alpha.1
	k8s.io/cli-runtime v0.23.0-alpha.1
	k8s.io/client-go v0.23.0-alpha.1
	k8s.io/klog/v2 v2.10.0 // indirect
	k8s.io/kube-openapi v0.0.0-20211115234752-e816edb12b65 // indirect
	k8s.io/kubectl v0.23.0-alpha.1
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b // indirect
	sigs.k8s.io/kustomize/api v0.10.1 // indirect
)
