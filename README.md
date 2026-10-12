# Pook

Pook is a Kubernetes operator for configuring Ceph RBD mirroring between two existing Ceph clusters.

It uses an `RBDMirror` custom resource to describe cluster connections and pool mirroring settings. The controller reads connection information from Kubernetes Secrets, observes Ceph configuration, and reconciles pool mirroring and peer relationships.

## Project Status

**Pook is under active development and is not ready for production use.**

The current implementation is an early foundation. Many features are planned, and substantial functionality remains to be implemented. The API, configuration, and behavior may change as development progresses.

Use Pook only in isolated development and test environments.

## Current Scope

- Kubernetes custom resources for mirroring configuration.
- Ceph connection settings stored in Kubernetes Secrets.
- Cluster identity and pool configuration checks.
- Pool-level mirroring configuration.
- Peer bootstrap operations.
- Reconciliation status reported through resource conditions.

## Planned Work

- Management of `rbd-mirror` daemons.
- Replication health monitoring and readiness reporting.
- Expanded peer lifecycle management.
- Explicit ownership and cleanup policies.
- Broader unit and integration test coverage.
- End-to-end validation against Ceph clusters.
- A documented Kubernetes and Ceph compatibility matrix.
- Versioned releases and installation artifacts.

These features are planned work, not capabilities of the current implementation.

## Prerequisites

For development and testing:

- Go matching the version declared in `go.mod`.
- Docker or a compatible container build tool.
- `kubectl` and access to a development Kubernetes cluster.
- Permission to install CRDs and the operator's RBAC resources.
- Two Ceph clusters reachable from the operator.
- An existing pool with the same name on both Ceph clusters.
- Ceph client credentials authorized for the required operations.

Testing actual data replication also requires separately managed `rbd-mirror` daemons and appropriately configured RBD images.

## Build and Deploy

Clone the repository:

```sh
git clone https://github.com/BenyaminSam/Pook.git
cd Pook
```

Set an image repository that you can push to:

```sh
export IMG=registry.example.com/your-team/pook:dev
```

Build, push, and deploy:

```sh
make docker-build docker-push IMG="$IMG"
make install
make deploy IMG="$IMG"
```

Check the deployment:

```sh
kubectl rollout status \
  deployment/pook-controller-manager \
  -n pook-system
```

The Kubernetes nodes must be able to pull the image. Configure image pull credentials separately when using a private registry.

## Example

This example configures bidirectional mirroring for an existing pool named `replicated-pool`.

Run it only against test clusters. Applying an `RBDMirror` allows the controller to modify Ceph configuration.

### 1. Create a Namespace

```sh
kubectl create namespace pook-demo
```

### 2. Create Connection Secrets

Prepare a configuration file and client keyring for each cluster:

```text
local/
  ceph.conf
  ceph.client.pook.keyring

peer/
  ceph.conf
  ceph.client.pook.keyring
```

Create the Secrets:

```sh
kubectl -n pook-demo create secret generic pook-local-ceph \
  --from-file=ceph.conf=./local/ceph.conf \
  --from-file=keyring=./local/ceph.client.pook.keyring

kubectl -n pook-demo create secret generic pook-peer-ceph \
  --from-file=ceph.conf=./peer/ceph.conf \
  --from-file=keyring=./peer/ceph.client.pook.keyring
```

Secrets must be in the same namespace as the `RBDMirror` resource. The required data keys are `ceph.conf` and `keyring`.

Do not commit credentials, keyrings, or bootstrap tokens to version control.

### 3. Create an RBDMirror

Save the following as `rbdmirror.yaml`.

Replace the cluster IDs with your actual Ceph FSIDs. Adjust the client and pool names to match your environment.

```yaml
apiVersion: storage.pook.io/v1alpha1
kind: RBDMirror
metadata:
  name: example
  namespace: pook-demo
spec:
  localCluster:
    clusterID: "<local-ceph-fsid>"
    clientName: client.pook
    configSecretRef:
      name: pook-local-ceph
    credentialsSecretRef:
      name: pook-local-ceph

  peerCluster:
    clusterID: "<peer-ceph-fsid>"
    clientName: client.pook
    configSecretRef:
      name: pook-peer-ceph
    credentialsSecretRef:
      name: pook-peer-ceph

  pools:
    - name: replicated-pool
      mode: journal
      direction: rx-tx
```

Apply the resource:

```sh
kubectl apply -f rbdmirror.yaml
```

### 4. Inspect Reconciliation

```sh
kubectl -n pook-demo get rbdmirror example -o yaml

kubectl -n pook-demo describe rbdmirror example

kubectl -n pook-system logs \
  deployment/pook-controller-manager \
  -c manager \
  --tail=100
```

Resource conditions describe the controller's configuration assessment. Verify actual data replication independently with Ceph tooling; replication-health reporting is planned work.

## Development

Generate manifests and code:

```sh
make manifests generate
```

Format, test, and lint:

```sh
make fmt vet
make test
make lint
```

List available commands:

```sh
make help
```

The test target uses envtest and downloads Kubernetes test binaries.

To run the controller locally:

```sh
make run
```

Local execution uses your current Kubernetes context and requires the Ceph CLI tools. Use a dedicated development context and test Ceph clusters.

## Repository Layout

```text
api/v1alpha1/        Custom resource API definitions
cmd/                 Controller manager entry point
internal/controller/ Reconciliation, planning, and status
internal/ceph/       Ceph CLI execution and connection handling
internal/kubernetes/ Kubernetes Secret helpers
config/              CRDs, RBAC, deployment, and samples
hack/                Development support files
```

Generate CRDs and RBAC with `make manifests`, and DeepCopy implementations with `make generate`. Do not edit generated files directly.

## Uninstall

Deleting an `RBDMirror` does not undo the Ceph configuration created during reconciliation. Manage existing peer relationships and mirrored data separately.

Remove the example resource:

```sh
kubectl -n pook-demo delete rbdmirror example
```

Remove the controller:

```sh
make undeploy
```

Remove the CRD only when all `RBDMirror` resources are no longer needed:

```sh
make uninstall
```

Removing the CRD deletes all remaining `RBDMirror` resources across the Kubernetes cluster.

Delete connection Secrets separately when they are no longer required.

## Contributing

The project is evolving, and focused contributions are welcome.

For changes to controller behavior, include tests covering the affected reconciliation paths. Regenerate manifests and code when needed, and run tests and linting before submitting a pull request.

When reporting an issue, include:

- The project commit or version.
- Kubernetes and Ceph versions.
- Steps to reproduce the behavior.
- Sanitized resource conditions and logs.

Never include credentials, keyrings, or bootstrap tokens.