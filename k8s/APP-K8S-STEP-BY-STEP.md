# NoppFlow: Step-by-step guide to register an App and deploy to Kubernetes

This guide covers the full path from zero to first successful deployment.

## 1) Prerequisites

1. NoppFlow running and reachable in your browser.
2. Kubernetes cluster reachable from the environment where NoppFlow runs.
3. Runner image available and pullable by the cluster (`k8s_runner_image` field).
4. App Git repository containing:
   - a `Dockerfile` (if you build an image),
   - Kubernetes manifests (`k8s/*.yaml`) for `kubectl` mode, or a chart for `helm` mode.

## 2) Configure Kubernetes first (required before app registration)

Important:
1. NoppFlow does not use end-user kubeconfig for deploy.
2. Deploy is executed with cluster context + ServiceAccount/RBAC.

### 2.1 Create namespaces (example)

```bash
kubectl create namespace noppflow --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace apps --dry-run=client -o yaml | kubectl apply -f -
```

### 2.2 Ensure controller RBAC for NoppFlow

This allows NoppFlow API pod to create temporary Jobs/Secrets for `k8s_deploy`.

```bash
kubectl apply -f k8s/controller-serviceaccount.example.yaml
kubectl apply -f k8s/controller-rbac.namespace.example.yaml
```

If needed, edit:
1. `metadata.namespace` (apps namespace),
2. `subjects[].namespace` (namespace where NoppFlow runs).

### 2.3 Create app runner ServiceAccount and RBAC

This defines deploy permissions granted to the app in target namespace.

```bash
kubectl apply -f k8s/runner-rbac.example.yaml
```

By default this creates:
1. `ServiceAccount`: `noppflow-runner` in namespace `apps`,
2. `Role` + `RoleBinding` for deployments/services/configmaps/secrets/jobs etc.

### 2.4 Validate permissions

At least validate:

```bash
kubectl auth can-i create jobs -n apps --as=system:serviceaccount:noppflow:noppflow-controller
kubectl auth can-i create deployments -n apps --as=system:serviceaccount:apps:noppflow-runner
kubectl auth can-i patch deployments -n apps --as=system:serviceaccount:apps:noppflow-runner
```

If any command returns `no`, fix Role/RoleBinding before continuing.

## 3) Login and initial setup in NoppFlow

1. Login as `admin`.
2. Open `Access`.
3. Register an SSH key:
   - `Name`: clear name (example: `github-main`).
   - `Private key`: key with access to the repo.
   - For public HTTPS repo, you may use a placeholder key if your policy allows it.
4. (Optional) Register global env vars in `Global Env Vars`:
   - Example: `REGISTRY_ADDR`, `ENVIRONMENT`, `TEAM_NAME`.
   - They are available inside steps (`$VAR_NAME`).

## 4) Create the App (Apps -> Add app)

Fill the fields:

1. **Name**: friendly app name.
2. **Repository URL**: git URL (`https://...` or `git@...`).
3. **Branch**: deploy branch (example: `main`).
4. **SSH key**: select the key created above.

### Deploy mode

Choose based on your deployment strategy:

1. `kubectl`:
   - `Kubernetes namespace`: target namespace (example: `apps`).
   - `Kubernetes service account`: runner SA (example: `noppflow-runner`).
   - `Runner image`: runner image (example: `localhost:5001/noppflow-runner:latest`).
   - `Manifest path`: manifests path (example: `k8s/`).
2. `helm`:
   - Same namespace/SA/runner image.
   - `Helm chart`: chart path in repo (example: `charts/my-app`).
   - `Helm values path` (optional): example `charts/my-app/values-prod.yaml`.

## 5) Define pipeline steps

Recommended sequence:

1. `precheck` (`cmd`):
   - validate files/context.
   - example: `echo precheck && test -f Dockerfile`
2. `build_push` (`cmd`):
   - build and push image.
   - run-tag example:
     - `IMAGE_TAG=run-$NOPPFLOW_RUN_ID`
     - `IMAGE_REF=<registry>/<app>:$IMAGE_TAG`
     - build/push using your tool (kaniko/buildah/docker).
3. `deploy_apply` (`k8s_deploy`):
   - deploy step executed with configured `deploy_mode`.
4. `deploy_set_image` (`cmd`, optional but common):
   - update deployment image.
   - example:
     - `kubectl -n apps set image deploy/my-app my-app=<registry>/my-app:run-$NOPPFLOW_RUN_ID`
5. `deploy_rollout` (`cmd`):
   - wait for rollout completion.
   - example:
     - `kubectl -n apps rollout status deploy/my-app --timeout=180s`

Notes:

1. Each step must have only one mode (`cmd`, `file`, `script`, or `k8s_deploy`).
2. `sleep_sec` is optional (0..3600).
3. Save app at the end.

## 6) Run first deploy

1. Go to `Apps`.
2. Click `Run` on your app.
3. Open `Recent runs`.
4. Expand run row (arrow).
5. Follow:
   - live log (auto-follow),
   - step rail state (`waiting`, `running`, `success`, `failed`).

## 7) Validate in cluster

Useful commands:

```bash
kubectl -n <namespace> get deploy,po
kubectl -n <namespace> rollout status deploy/<deployment-name>
kubectl -n <namespace> get events --sort-by=.metadata.creationTimestamp | tail -n 30
```

If you used `set image`, confirm final image:

```bash
kubectl -n <namespace> get deploy <deployment-name> -o jsonpath='{.spec.template.spec.containers[*].image}{"\n"}'
```

## 8) Quick troubleshooting

### 1. `kubectl: not found` in runner

1. Confirm `k8s_runner_image` value.
2. Move to a newer runner image.
3. Verify cluster can pull that image.

### 2. `selector does not match template labels`

1. Fix Deployment manifest.
2. `spec.selector.matchLabels` must match `spec.template.metadata.labels` exactly.

### 3. `ErrImagePull`

1. Check registry address (push vs pull address).
2. Confirm image tag exists.
3. Confirm node network access to registry.

### 4. Step finishes but app does not start

1. Run `kubectl describe pod`.
2. Validate probes, env vars, and image.
3. Validate namespace and service account.

### 5. Private repo clone fails

1. Confirm app `ssh_key_name`.
2. Confirm key permission to repo/branch.
3. Verify repo URL and branch.

## 9) Minimal step template (kubectl)

```text
1) precheck (cmd)
echo precheck && test -f k8s/deployment.yaml

2) build_push (cmd)
IMAGE_TAG=run-$NOPPFLOW_RUN_ID
IMAGE_REF=<registry>/my-app:$IMAGE_TAG
echo "build/push $IMAGE_REF"

3) deploy_apply (k8s_deploy)

4) deploy_set_image (cmd)
kubectl -n apps set image deploy/my-app my-app=<registry>/my-app:run-$NOPPFLOW_RUN_ID

5) deploy_rollout (cmd)
kubectl -n apps rollout status deploy/my-app --timeout=180s
```

## 10) Practical adoption tip

1. Start with a simple sample repo.
2. Close base pipeline first (`precheck` + `deploy_apply` + `rollout`).
3. Then add build/push with `NOPPFLOW_RUN_ID` image versioning.
