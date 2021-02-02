package app

import "github.com/puppetlabs/leg/k8sutil/pkg/controller/ownerext"

var DependencyManager = ownerext.NewManager("pvpool.puppet.com/owner")
