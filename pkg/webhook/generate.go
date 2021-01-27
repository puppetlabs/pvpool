//go:generate go run sigs.k8s.io/controller-tools/cmd/controller-gen rbac:roleName=pvpool-webhook object webhook paths=./... output:artifacts:config=../../manifests/webhook/generated

package webhook
