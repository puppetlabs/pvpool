module github.com/puppetlabs/pvpool

go 1.14

require (
	github.com/golangci/golangci-lint v1.36.0
	github.com/google/uuid v1.1.2
	github.com/puppetlabs/leg/errmap v0.1.0
	github.com/puppetlabs/leg/k8sutil v0.1.1-0.20210129194109-f5cb96b0bf5b
	github.com/puppetlabs/leg/mainutil v0.1.2
	github.com/puppetlabs/leg/mathutil v0.1.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	gotest.tools/gotestsum v1.6.1
	k8s.io/api v0.20.2
	k8s.io/apimachinery v0.20.2
	k8s.io/client-go v0.20.2
	k8s.io/klog/v2 v2.4.0
	sigs.k8s.io/controller-runtime v0.8.1
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.9.2
)

replace (
	k8s.io/api => k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.2
	k8s.io/client-go => k8s.io/client-go v0.19.2
)
