# PVPool [![CI](https://github.com/puppetlabs/pvpool/workflows/CI/badge.svg)](https://github.com/puppetlabs/pvpool/actions?query=workflow%3ACI) [![Go Report Card](https://goreportcard.com/badge/github.com/puppetlabs/pvpool)](https://goreportcard.com/report/github.com/puppetlabs/pvpool)

PVPool is a Kubernetes operator that preallocates a collection of [persistent volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/) with a requested size. By preallocating, applications are able to acquire storage more rapidly than underlying provisioners are able to fulfill.

Additionally, PVPool can configure the volumes with a job, allowing them to be prepopulated with application-specific data.

## Terminology

PVPool exposes two new Kubernetes resources:

* `Pool`: A collection of PVs. The PVPool controller tries to guarantee that the exact number of PVs specified by the `replicas` field of a pool spec are in the pool at any given time.
* `Checkout`: A request to take a single PV from a referenced pool as a PVC. Once the PV is checked out from the pool, the pool will automatically create a new PV to take its place.

## Installation

PVPool is currently under development and there is no official release. For now, you should follow the [instructions for contributing to PVPool](CONTRIBUTING.md).

## Usage

If you're using Rancher's [Local Path Provisioner](https://github.com/rancher/local-path-provisioner) (or have a storage class named `local-path`), you can create the pools and checkouts in the `examples` directory without any modifications. You should end up with a set of PVCs with names starting with `test-pool-`, corresponding PVs, plus a checked out PVC starting with the name `test-checkout-a`.

### Storage class requirements and limitations

PVPool doesn't really understand storage classes that have `volumeBindingMode: "WaitForFirstConsumer"` in the sense that they're described in the Kubernetes documentation. Rather, we always ensure the PVC is bound before putting it into the pool. We do this using a job, though, so any special requirements around how pods are created (e.g., node taints) will be respected.

You should be careful using storage classes that have a `reclaimPolicy` other than `"Delete"`. If you do, take note that there are no restrictions on churning through many checkouts, so you may find yourself accumulating lots of stale persistent volumes.

### Prepopulating volumes

Here's a pool with an init job that writes some data to the PV before making it available to be checked out:

```yaml
apiVersion: pvpool.puppet.com/v1alpha1
kind: Pool
metadata:
  name: test-pool-with-init-job
spec:
  replicas: 5
  selector:
    matchLabels:
      app.kubernetes.io/name: pvpool-test-with-init-job
  template:
    metadata:
      labels:
        app.kubernetes.io/name: pvpool-test-with-init-job
    spec:
      storageClassName: local-path
      resources:
        requests:
          storage: 50Mi
  initJob:
    template:
      spec:
        backoffLimit: 2
        activeDeadlineSeconds: 60
        template:
          spec:
            containers:
            - name: init
              image: busybox:stable-musl
              command:
              - /bin/sh
              - -c
              - |
                echo 'Wow, such prepopulated!' >/workspace/data.txt
              volumeMounts:
              - name: my-volume
                mountPath: /workspace
    volumeName: my-volume
```

When you use init jobs with PVPool, note that the pod `restartPolicy` will always be `Never` and that the job `backoffLimit` and `activeDeadlineSeconds` are limited to 10 and 600, respectively. If you don't specify a `volumeName` in the `initJob`, it will default to `"workspace"`. Volumes are always automatically added to the pod spec, but you must provide the relevant mount path for each container you want to use the volume with.

### RBAC

PVPool takes advantage of a lesser-known Kubernetes RBAC verb, `"use"`, to ensure the creator of a checkout has access to the pool they've requested. This allows the pool to exist opaquely, perhaps even in another namespace, while still allowing a user with little trust to provision the storage they need.

For example, given the following checkout object:

```yaml
apiVersion: pvpool.puppet.com/v1alpha1
kind: Checkout
metadata:
  namespace: restricted
  name: my-checkout
spec:
  poolRef:
    namespace: storage
    name: restricted-pool
```

The user creating the checkout will need the following roles bound:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: restricted
  name: checkout-owner
rules:
- apiGroups: [pvpool.puppet.com/v1alpha1]
  resources: [checkouts]
  verbs: [get, list, watch, create, update, patch, delete]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  namespace: storage
  name: restricted-pool-user
rules:
- apiGroups: [pvpool.puppet.com/v1alpha1]
  resources: [pools]
  resourceNames: [restricted-pool]
  verbs: [use]
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for more information on how to contribute to this project.
