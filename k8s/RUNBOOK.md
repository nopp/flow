# NoppFlow Kubernetes Runbook

This runbook covers day-to-day operation of ephemeral Job runs (`k8s_deploy`) in NoppFlow.

## 1. Preconditions

- NoppFlow deployed in namespace `noppflow`.
- `kubectl` available inside NoppFlow API container.
- NoppFlow API ServiceAccount configured (`noppflow-controller`).
- RBAC applied for controller (namespace-scoped recommended).
- Runner ServiceAccount + RBAC applied in target app namespaces.
- App has valid fields:
  - `ssh_key_name` (required only for SSH repos like `git@...` / `ssh://`)
  - `deploy_mode` (`kubectl` or `helm`)
  - `k8s_namespace`
  - `k8s_service_account`
  - `k8s_runner_image`
  - mode-specific path/chart settings

## 2. First-time Setup Checklist

1. Apply controller SA:
   ```bash
   kubectl apply -f k8s/controller-serviceaccount.example.yaml
   ```
2. Apply controller RBAC (per namespace preferred):
   ```bash
   kubectl apply -f k8s/controller-rbac.namespace.example.yaml
   ```
3. Apply runner RBAC in app namespace(s):
   ```bash
   kubectl apply -f k8s/runner-rbac.example.yaml
   ```
4. Deploy/rollout NoppFlow:
   ```bash
   kubectl apply -f k8s/deployment.yaml
   kubectl apply -f k8s/service.yaml
   kubectl -n noppflow rollout status deploy/noppflow
   ```
5. Confirm NoppFlow pod uses `noppflow-controller`:
   ```bash
   kubectl -n noppflow get pod -l app=noppflow -o jsonpath='{.items[0].spec.serviceAccountName}'
   ```

## 3. Trigger and Validate a Run

1. In UI, open app and trigger run.
2. Confirm Job exists in app namespace:
   ```bash
   kubectl -n <app-namespace> get jobs | grep noppflow-run-
   ```
3. Check pod logs directly if needed:
   ```bash
   kubectl -n <app-namespace> logs job/<job-name>
   ```
4. Confirm NoppFlow run reaches `success` or `failed` and logs are visible in UI.

## 4. Routine Operations

- List recent runner jobs:
  ```bash
  kubectl -n <app-namespace> get jobs --sort-by=.metadata.creationTimestamp
  ```
- Remove stuck/old jobs manually:
  ```bash
  kubectl -n <app-namespace> delete job <job-name>
  ```
- Confirm temporary SSH secret cleanup:
  ```bash
  kubectl -n <app-namespace> get secrets | grep noppflow-run-
  ```

## 5. Troubleshooting

## 5.1 Run fails immediately with RBAC errors

Symptoms:
- run log shows `forbidden` on jobs/secrets/pods

Checks:
```bash
kubectl -n <app-namespace> auth can-i create jobs --as=system:serviceaccount:noppflow:noppflow-controller
kubectl -n <app-namespace> auth can-i create secrets --as=system:serviceaccount:noppflow:noppflow-controller
kubectl -n <app-namespace> auth can-i get pods/log --as=system:serviceaccount:noppflow:noppflow-controller
```

Fix:
- Re-apply controller RBAC for that namespace.

## 5.2 Job starts but step commands fail

Symptoms:
- pod logs contain `command not found`, `git: not found`, `kubectl: not found`, or `helm: not found`

Fix:
- Update `k8s_runner_image` to include required tooling.

## 5.3 Git clone fails in runner pod

Symptoms:
- `Permission denied (publickey)`

Checks:
- app `ssh_key_name` exists and key is valid.
- runner pod has mounted secret `/var/run/noppflow-ssh/id_key`.

Fix:
- rotate/recreate SSH key in NoppFlow and re-save app config.

For HTTPS public repos, `ssh_key_name` is not required. If clone fails with auth prompts, verify repository URL/access and branch.

## 5.4 Deploy step (`k8s_deploy`) fails

Checks:
- `deploy_mode` is correct.
- `deploy_manifest_path` exists (kubectl mode).
- `helm_chart` exists and `helm_values_path` (if used) is valid.
- runner service account has namespace permissions to apply target resources.

## 6. Security Notes

- Prefer namespace-scoped controller RBAC over cluster-wide.
- Use dedicated runner service account per namespace/team.
- Keep runner images minimal and pinned by tag/digest.
- Rotate SSH keys regularly.

## 7. Quick Diagnostic Commands

```bash
kubectl -n noppflow get deploy,pod,sa
kubectl -n <app-namespace> get job,pod,sa,role,rolebinding
kubectl -n <app-namespace> describe job <job-name>
kubectl -n <app-namespace> logs job/<job-name>
```
