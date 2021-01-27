//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:preserveUnknownFields=false rbac:roleName=pvpool-controller object paths=./... output:crd:artifacts:config=../manifests/crd/generated output:rbac:artifacts:config=../manifests/controller/generated

package pkg
