module github.com/puppetlabs/pvpool

go 1.14

require (
	github.com/google/uuid v1.1.2
	github.com/puppetlabs/leg/k8sutil v0.0.0-00010101000000-000000000000
	github.com/puppetlabs/leg/mainutil v0.1.2
	github.com/puppetlabs/leg/mathutil v0.1.0
	k8s.io/api v0.19.2
	k8s.io/apimachinery v0.20.1
	sigs.k8s.io/controller-runtime v0.7.0
	sigs.k8s.io/controller-tools v0.4.1
)

replace (
	k8s.io/api => k8s.io/api v0.19.2
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.19.2
	k8s.io/apimachinery => k8s.io/apimachinery v0.19.2
	k8s.io/client-go => k8s.io/client-go v0.19.2
)

replace github.com/puppetlabs/leg/k8sutil => ../leg/k8sutil
