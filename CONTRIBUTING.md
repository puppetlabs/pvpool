# Contributing to PVPool

## Installing PVPool from source

To use your local copy of PVPool on a Kubernetes cluster, you'll need Google's [ko](https://github.com/google/ko) tool and a Docker registry accessible from both your local machine and your Kubernetes cluster.

If you're developing locally, it's probably easiest to get started with [k3d](https://k3d.io), especially if you're using Linux. Create a registry and a cluster:

```
$ k3d registry create registry.localhost --port 5000
$ mkdir -p /tmp/local-path-provisioner-data
$ k3d cluster create pvpool-test \
    --registry-use k3d-registry.localhost:5000 \
    --volume /tmp/local-path-provisioner-data:/opt/local-path-provisioner:shared
$ export KO_DOCKER_REPO=k3d-registry.localhost:5000
```

Our own [CI workflow](.github/workflows/ci.yaml) may be a useful reference.

Once you have ko and a registry, you can build and install PVPool in a few ways. Each installation option uses your current `$KUBECONFIG` to determine which cluster to act on.

* Release: The PVPool controller, webhook, and automatic certificate generation as designed for packaging for general consumption.

  * Build: `make build-manifest-release`
  * Install: `make apply-release`

* Debug: Same as release, but the logs for all PVPool pods are set to provide extra verbosity.

  * Build: `make build-manfiest-debug`
  * Install: `make apply-debug` (or just `make apply`)

* Test: Same as debug, but Rancher's [Local Path Provisioner](https://github.com/rancher/local-path-provisioner) is also installed to make running end-to-end tests easier.

  * Build: `make build-manifest-test`
  * Install: `make apply-test`

When you make a code change you want to deploy, simply rerun the relevant build or apply command. You do not need to manually manage any Docker images.

## Running tests

By default, `make test` will only run unit tests.

If you'd like to run the end-to-end test suite using the default Local Path Provisioner, you need to set the environment variable `PVPOOL_TEST_E2E_KUBECONFIG` to the path to a Kubeconfig for a cluster acceptable for testing. `make test` understands this configuration and will automatically run the end-to-end tests.

If you want to use a different provisioner, you need to tell the test suite which storage class to use explicitly using the `PVPOOL_TEST_E2E_STORAGE_CLASS_NAME` environment variable. For example, if you're running the tests against a GKE cluster, you might want to set `PVPOOL_TEST_E2E_STORAGE_CLASS_NAME=standard`. In this configuration, the Local Path Provisioner will _not_ be installed.

**Important:** Do not run the tests against a cluster you care about! We haven't audited the tests to make sure they won't delete something they're not supposed to (and we may never do so!).

## Making changes

* Clone the repository into your own namespace.
* Always branch off of `main`, our default branch.
* Make commits of logical and atomic units.
* Check for unnecessary whitespace with `git diff --check` before committing.
* Make sure your commit messages are in the proper format. We (try to!) follow [Tim Pope's guidelines](https://tbaggery.com/2008/04/19/a-note-about-git-commit-messages.html) for writing good commit messages: format for short lines, use the imperative mood ("Add X to Y"), describe before and after state in the commit message body.
* Make sure you have added the necessary tests for your changes.
* Submit a pull request per the usual GitHub PR process.

## Additional resources

* [Puppet community guidelines](https://puppet.com/community/community-guidelines)
* [Puppet community Slack](https://slack.puppet.com)
* [Relay issue tracker](https://github.com/puppetlabs/relay/issues)
* [General GitHub documentation](https://help.github.com/)
* [GitHub pull request documentation](https://help.github.com/articles/creating-a-pull-request/)
