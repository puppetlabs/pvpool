//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen crd:preserveUnknownFields=false object paths=./... output:artifacts:config=../../manifests/crd/generated

package apis
