module github.com/puppetlabs/pvpool

go 1.16

require (
	github.com/golangci/golangci-lint v1.36.0
	github.com/google/uuid v1.1.2
	github.com/puppetlabs/leg/errmap v0.1.0
	github.com/puppetlabs/leg/k8sutil v0.5.0
	github.com/puppetlabs/leg/mainutil v0.1.2
	github.com/puppetlabs/leg/mathutil v0.1.0
	github.com/puppetlabs/leg/timeutil v0.3.0
	github.com/spf13/viper v1.7.1
	github.com/stretchr/testify v1.7.0
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	gotest.tools/gotestsum v1.6.1
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/controller-runtime v0.9.2
	sigs.k8s.io/controller-tools v0.4.1
	sigs.k8s.io/kustomize/kustomize/v3 v3.9.2
)
