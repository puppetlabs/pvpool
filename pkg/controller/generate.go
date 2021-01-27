//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=pvpool-controller paths=./... output:artifacts:config=../../manifests/controller/generated

package controller
