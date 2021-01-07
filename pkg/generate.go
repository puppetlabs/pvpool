//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:preserveUnknownFields=false rbac:roleName=pvpool-controller object paths=./... output:artifacts:config=../manifests/generated

package pkg
