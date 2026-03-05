# Kubernetes manifests for NoppFlow

## Prerequisites

- Build and push the container image (e.g. `noppflow:latest`) to your registry, or use a local image with `imagePullPolicy: IfNotPresent`.
- Database:
  - local bootstrap script uses SQLite by default
  - MySQL is optional for local and typical for production

## Local kind bootstrap (recommended for team testing)

Use the automated script to create a local kind cluster, local registry, build/push `noppflow` + `noppflow-runner`, deploy MySQL, RBAC, and NoppFlow:

```bash
./k8s/setup-kind-local.sh
```

The script also:
- forces `kubectl` context to `kind-<cluster-name>` (default: `kind-noppflow-local`)
- loads images directly into kind nodes (`kind load docker-image`) as fallback for local runtime/registry quirks

By default, it also seeds a test app in NoppFlow:
- name: `app-teste`
- repo: `https://github.com/nopp/app-teste.git`
- deploy mode: `kubectl` (namespace `apps`)
- if `app-teste` already exists, it is updated to current bootstrap settings (including `k8s_runner_image`)
- if `TEST_APP_BRANCH=main` but repo default branch is different, setup auto-detects and uses repo default branch

State reset:
- by default, local bootstrap resets `noppflow` and `apps` namespaces each run (`RESET_DATABASE=true`)
- this guarantees a fresh database and avoids accumulating duplicate test apps/runs
- to keep existing state:
  - `RESET_DATABASE=false ./k8s/setup-kind-local.sh`

Image tags:
- script uses a unique `APP_IMAGE_TAG` per run (timestamp-based) to avoid stale `:kind` image reuse in local clusters
- override if needed:
  - `APP_IMAGE_TAG=mytag ./k8s/setup-kind-local.sh`

To skip app seeding:

```bash
SEED_TEST_APP=false ./k8s/setup-kind-local.sh
```

Default local DB mode is `sqlite` for reliability in dev bootstrap.
To force MySQL mode:

```bash
DB_MODE=mysql ./k8s/setup-kind-local.sh
```

Using Podman:

```bash
podman machine stop
podman machine set --rootful --cpus 4 --memory 8192
podman machine start
CONTAINER_CLI=podman ./k8s/setup-kind-local.sh
```

Optional: pin kind node image explicitly (default already pinned for stability):

```bash
KIND_NODE_IMAGE=kindest/node:v1.30.8 CONTAINER_CLI=podman ./k8s/setup-kind-local.sh
```

Requirements:
- Docker or Podman
- kind
- kubectl

After setup:

```bash
kubectl -n noppflow port-forward svc/noppflow 8080:80
# open http://localhost:8080 (admin/admin)
```

## 1. Create the database secret

Create the secret with the MySQL DSN (replace with your values):

```bash
kubectl create secret generic noppflow-db \
  --from-literal=dsn='user:password@tcp(mysql-host:3306)/noppflow?parseTime=true'
```

See `secret.example.yaml` for more options.

## 2. Deploy

```bash
kubectl apply -f k8s/controller-serviceaccount.example.yaml
kubectl apply -f k8s/deployment.yaml
kubectl apply -f k8s/service.yaml
```

## 3. Access

The Service exposes NoppFlow on port 80 (ClusterIP). To access from outside the cluster, use a port-forward or add an Ingress:

```bash
kubectl port-forward service/noppflow 8080:80
# Then open http://localhost:8080
```

## Customization

- **Image**: Update `image` in `deployment.yaml` to your registry/image:tag.
- **Replicas**: Change `spec.replicas` in `deployment.yaml`.
- **Config / work dir**: Uncomment the volumes and volumeMounts in the deployment if you need to mount `config/apps.yaml` or a PVC for the `work` directory (clone output).

## Runner RBAC and Job Template

For app-level deploy inside the cluster, use:

- `runner-rbac.example.yaml`: service account + namespace-scoped role/binding for deploy commands.
- `runner-job.example.yaml`: example ephemeral job to run steps and deploy using `kubectl`.

NoppFlow can now create ephemeral runner Jobs automatically for apps that include a `k8s_deploy` step.
In app settings, set:
- `k8s_namespace`
- `k8s_service_account`
- `k8s_runner_image`
- `deploy_mode` (`kubectl` or `helm`)

## Controller RBAC (NoppFlow API Pod)

NoppFlow API needs permission to create and monitor ephemeral Jobs and temporary SSH Secrets.

### Recommended: namespace-scoped RBAC

Apply `controller-rbac.namespace.example.yaml` in each app namespace (adjust namespace names first):

```bash
kubectl apply -f k8s/controller-rbac.namespace.example.yaml
```

### Alternative: cluster-wide RBAC

If apps run in many namespaces and you want a single binding:

```bash
kubectl apply -f k8s/controller-rbac.cluster.example.yaml
```

This is broader; prefer namespace-scoped where possible.

Apply after adjusting namespace/image/permissions:

```bash
kubectl apply -f k8s/runner-rbac.example.yaml
kubectl apply -f k8s/runner-job.example.yaml
```

Operational checklist and troubleshooting:

- `k8s/RUNBOOK.md`
