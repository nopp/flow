#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-noppflow-local}"
KUBE_CONTEXT="${KUBE_CONTEXT:-kind-${CLUSTER_NAME}}"
REGISTRY_NAME="${REGISTRY_NAME:-kind-registry}"
REGISTRY_PORT="${REGISTRY_PORT:-5001}"
REGISTRY_HOST="${REGISTRY_HOST:-localhost}"
REGISTRY_ADDR="${REGISTRY_HOST}:${REGISTRY_PORT}"
REGISTRY_IN_CLUSTER_ADDR="${REGISTRY_NAME}:5000"
CONTROLLER_NS="${CONTROLLER_NS:-noppflow}"
APPS_NS="${APPS_NS:-apps}"
APP_IMAGE_TAG="${APP_IMAGE_TAG:-$(date +%Y%m%d%H%M%S)}"
APP_IMAGE_REPO="${APP_IMAGE_REPO:-${REGISTRY_ADDR}/noppflow}"
RUNNER_IMAGE_REPO="${RUNNER_IMAGE_REPO:-${REGISTRY_ADDR}/noppflow-runner}"
APP_IMAGE="${APP_IMAGE:-${APP_IMAGE_REPO}:${APP_IMAGE_TAG}}"
RUNNER_IMAGE="${RUNNER_IMAGE:-${RUNNER_IMAGE_REPO}:${APP_IMAGE_TAG}}"
RUNNER_IMAGE_JOB="${RUNNER_IMAGE_JOB:-${RUNNER_IMAGE}}"
CONTAINER_CLI="${CONTAINER_CLI:-docker}"
KIND_NODE_IMAGE="${KIND_NODE_IMAGE:-kindest/node:v1.30.8}"
DB_MODE="${DB_MODE:-sqlite}"
RESET_DATABASE="${RESET_DATABASE:-true}"
ENABLE_INCLUSTER_REGISTRY="${ENABLE_INCLUSTER_REGISTRY:-true}"
INCLUSTER_REGISTRY_NAME="${INCLUSTER_REGISTRY_NAME:-noppflow-registry}"
INCLUSTER_REGISTRY_ADDR="${INCLUSTER_REGISTRY_NAME}.${APPS_NS}.svc.cluster.local:5000"
APP_RUNTIME_REGISTRY_ADDR="${APP_RUNTIME_REGISTRY_ADDR:-}"
if [ -z "${APP_RUNTIME_REGISTRY_ADDR}" ]; then
  APP_RUNTIME_REGISTRY_ADDR="${REGISTRY_IN_CLUSTER_ADDR}"
fi
APP_RUNTIME_REGISTRY_PUSH_ADDR="${APP_RUNTIME_REGISTRY_PUSH_ADDR:-${APP_RUNTIME_REGISTRY_ADDR}}"
APP_RUNTIME_REGISTRY_PULL_ADDR="${APP_RUNTIME_REGISTRY_PULL_ADDR:-${REGISTRY_ADDR}}"
SEED_TEST_APP="${SEED_TEST_APP:-true}"
TEST_APP_NAME="${TEST_APP_NAME:-app-teste}"
TEST_APP_REPO="${TEST_APP_REPO:-https://github.com/nopp/app-teste.git}"
TEST_APP_BRANCH="${TEST_APP_BRANCH:-main}"
TEST_SSH_KEY_NAME="${TEST_SSH_KEY_NAME:-public-http}"
LOCAL_API_PORT="${LOCAL_API_PORT:-18080}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "error: required command not found: $1" >&2
    exit 1
  fi
}

push_image() {
  local image="$1"
  if [ "${CONTAINER_CLI}" = "podman" ]; then
    "${CONTAINER_CLI}" push --tls-verify=false "${image}"
  else
    "${CONTAINER_CLI}" push "${image}"
  fi
}

require_container_runtime() {
  if ! "${CONTAINER_CLI}" info >/dev/null 2>&1; then
    if [ "${CONTAINER_CLI}" = "podman" ]; then
      cat >&2 <<'MSG'
error: Podman is not running/ready.

Start Podman machine and rerun:
- podman machine start

Then run:
- CONTAINER_CLI=podman ./k8s/setup-kind-local.sh
MSG
    else
      cat >&2 <<'MSG'
error: Docker daemon is not running.

Start Docker and rerun:
- Docker Desktop (macOS): open -a Docker
- Colima: colima start
MSG
    fi
    exit 1
  fi
}

require_podman_rootful() {
  if [ "${CONTAINER_CLI}" != "podman" ]; then
    return
  fi
  if ! podman machine inspect 2>/dev/null | grep -q '"Rootful": true'; then
    cat >&2 <<'MSG'
error: kind with Podman requires rootful podman machine in most environments.

Fix:
1) podman machine stop
2) podman machine set --rootful --cpus 4 --memory 8192
3) podman machine start
4) rerun: CONTAINER_CLI=podman ./k8s/setup-kind-local.sh
MSG
    exit 1
  fi
}

create_kind_cluster() {
  local kind_config
  local incluster_registry_patch=""
  if [ "${ENABLE_INCLUSTER_REGISTRY}" = "true" ]; then
    incluster_registry_patch=$(cat <<EOF
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${INCLUSTER_REGISTRY_ADDR}"]
      endpoint = ["http://${INCLUSTER_REGISTRY_ADDR}"]
EOF
)
  fi
  kind_config="$(mktemp)"
  cat > "${kind_config}" <<CFG
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
containerdConfigPatches:
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_ADDR}"]
      endpoint = ["http://${REGISTRY_NAME}:5000"]
  - |-
    [plugins."io.containerd.grpc.v1.cri".registry.mirrors."${REGISTRY_IN_CLUSTER_ADDR}"]
      endpoint = ["http://${REGISTRY_NAME}:5000"]
${incluster_registry_patch}
nodes:
  - role: control-plane
CFG
  kind create cluster --name "${CLUSTER_NAME}" --image "${KIND_NODE_IMAGE}" --config "${kind_config}"
  rm -f "${kind_config}"
}

load_kind_image_if_possible() {
  local image="$1"
  if kind load docker-image --name "${CLUSTER_NAME}" "${image}"; then
    return 0
  fi

  # Podman environments may not expose images in a way `kind load docker-image` can access.
  # Fallback: save image as archive and load it explicitly.
  if [ "${CONTAINER_CLI}" = "podman" ]; then
    local archive
    archive="$(mktemp "/tmp/kind-image-archive-XXXXXX.tar")"
    if podman save -o "${archive}" "${image}" && kind load image-archive --name "${CLUSTER_NAME}" "${archive}"; then
      rm -f "${archive}"
      return 0
    fi
    rm -f "${archive}" || true
  fi

  echo "- warning: could not preload ${image} into kind; continuing with registry pull path"
}

configure_kind_registry_http() {
  local hostport="$1"
  local node

  if [ -z "${hostport}" ]; then
    return 0
  fi

  echo "- configuring kind nodes for HTTP registry ${hostport}"
  for node in $(kind get nodes --name "${CLUSTER_NAME}"); do
    "${CONTAINER_CLI}" exec "${node}" /bin/sh -lc "mkdir -p '/etc/containerd/certs.d/${hostport}' && printf '%s\n' 'server = \"http://${hostport}\"' '[host.\"http://${hostport}\"]' '  capabilities = [\"pull\", \"resolve\", \"push\"]' '  skip_verify = true' > '/etc/containerd/certs.d/${hostport}/hosts.toml'"
    "${CONTAINER_CLI}" exec "${node}" /bin/sh -lc "kill -HUP \$(pidof containerd) >/dev/null 2>&1 || true"
  done
}

for cmd in "${CONTAINER_CLI}" kind kubectl; do
  require_cmd "$cmd"
done
require_cmd curl
require_container_runtime
require_podman_rootful

if [ "${CONTAINER_CLI}" = "podman" ]; then
  export KIND_EXPERIMENTAL_PROVIDER="${KIND_EXPERIMENTAL_PROVIDER:-podman}"
fi

PF_PID=""
COOKIE_FILE=""
cleanup_seed() {
  if [ -n "${PF_PID}" ] && kill -0 "${PF_PID}" >/dev/null 2>&1; then
    kill "${PF_PID}" >/dev/null 2>&1 || true
    wait "${PF_PID}" 2>/dev/null || true
  fi
  if [ -n "${COOKIE_FILE}" ] && [ -f "${COOKIE_FILE}" ]; then
    rm -f "${COOKIE_FILE}"
  fi
}
trap cleanup_seed EXIT

api_get() {
  local path="$1"
  curl -fsS -b "${COOKIE_FILE}" "http://127.0.0.1:${LOCAL_API_PORT}${path}"
}

api_post_json() {
  local path="$1"
  local payload="$2"
  local expected="${3:-201}"
  local code
  code="$(curl -sS -o /tmp/noppflow-api-resp.json -w "%{http_code}" -b "${COOKIE_FILE}" -H "Content-Type: application/json" -d "${payload}" "http://127.0.0.1:${LOCAL_API_PORT}${path}")"
  if [ "${code}" != "${expected}" ]; then
    echo "error: API POST ${path} returned ${code}, expected ${expected}" >&2
    cat /tmp/noppflow-api-resp.json >&2 || true
    exit 1
  fi
}

api_put_json() {
  local path="$1"
  local payload="$2"
  local expected="${3:-200}"
  local code
  code="$(curl -sS -o /tmp/noppflow-api-resp.json -w "%{http_code}" -X PUT -b "${COOKIE_FILE}" -H "Content-Type: application/json" -d "${payload}" "http://127.0.0.1:${LOCAL_API_PORT}${path}")"
  if [ "${code}" != "${expected}" ]; then
    echo "error: API PUT ${path} returned ${code}, expected ${expected}" >&2
    cat /tmp/noppflow-api-resp.json >&2 || true
    exit 1
  fi
}

api_delete() {
  local path="$1"
  local expected="${2:-204}"
  local code
  code="$(curl -sS -o /tmp/noppflow-api-resp.json -w "%{http_code}" -X DELETE -b "${COOKIE_FILE}" "http://127.0.0.1:${LOCAL_API_PORT}${path}")"
  if [ "${code}" != "${expected}" ]; then
    echo "error: API DELETE ${path} returned ${code}, expected ${expected}" >&2
    cat /tmp/noppflow-api-resp.json >&2 || true
    exit 1
  fi
}

build_test_app_payload() {
  local branch_to_use="${TEST_APP_BRANCH}"
  local detected_branch=""
  local dollar='$'
  detected_branch="$(detect_repo_default_branch "${TEST_APP_REPO}")"
  if [ -n "${detected_branch}" ] && [ "${TEST_APP_BRANCH}" = "main" ] && [ "${detected_branch}" != "main" ]; then
    branch_to_use="${detected_branch}"
    echo "- detected default branch '${detected_branch}' for ${TEST_APP_REPO}; overriding TEST_APP_BRANCH=main" >&2
  fi

  cat <<JSON
{
  "name":"${TEST_APP_NAME}",
  "repo":"${TEST_APP_REPO}",
  "branch":"${branch_to_use}",
  "ssh_key_name":"${TEST_SSH_KEY_NAME}",
  "deploy_mode":"kubectl",
  "k8s_namespace":"${APPS_NS}",
  "k8s_service_account":"noppflow-runner",
  "k8s_runner_image":"${RUNNER_IMAGE_JOB}",
  "deploy_manifest_path":"k8s/",
  "steps":[
    {"name":"precheck","cmd":"echo deploying ${TEST_APP_NAME} && if [ -f k8s/deployment.yaml ]; then sed -i 's/app: .*/app: ${TEST_APP_NAME}/g' k8s/deployment.yaml; fi"},
    {"name":"build_push","cmd":"IMAGE_TAG=run-${dollar}NOPPFLOW_RUN_ID; IMAGE_REF=${APP_RUNTIME_REGISTRY_PUSH_ADDR}/${TEST_APP_NAME}:${dollar}IMAGE_TAG; test -f /workspace/repo/Dockerfile; echo building ${dollar}IMAGE_REF; kaniko-executor --context dir:///workspace/repo --dockerfile /workspace/repo/Dockerfile --destination ${dollar}IMAGE_REF --insecure --skip-tls-verify --insecure-pull && echo built ${dollar}IMAGE_REF"},
    {"name":"deploy_apply","k8s_deploy":true},
    {"name":"deploy_set_image","cmd":"kubectl -n ${APPS_NS} set image deploy/${TEST_APP_NAME} ${TEST_APP_NAME}=${APP_RUNTIME_REGISTRY_PULL_ADDR}/${TEST_APP_NAME}:run-${dollar}NOPPFLOW_RUN_ID"},
    {"name":"deploy_rollout","cmd":"kubectl -n ${APPS_NS} rollout status deploy/${TEST_APP_NAME} --timeout=180s"}
  ]
}
JSON
}

extract_app_id_by_name() {
  local apps_json="$1"
  local name="$2"
  echo "${apps_json}" | tr '{' '\n' | grep "\"name\":\"${name}\"" | sed -n 's/.*"id":"\([^"]*\)".*/\1/p' | head -n1
}

extract_all_app_ids() {
  local apps_json="$1"
  echo "${apps_json}" | grep -o '"id":"[^"]*"' | sed 's/"id":"\([^"]*\)"/\1/' | sort -u
}

detect_repo_default_branch() {
  local repo="$1"
  if ! command -v git >/dev/null 2>&1; then
    echo ""
    return 0
  fi
  git ls-remote --symref "${repo}" HEAD 2>/dev/null \
    | sed -n 's#^ref: refs/heads/\([^[:space:]]*\)[[:space:]]*HEAD$#\1#p' \
    | head -n1
}

set_image_with_retry() {
  local attempts=3
  local i
  local out_file

  for i in $(seq 1 "${attempts}"); do
    out_file="$(mktemp)"
    if kubectl -n "${CONTROLLER_NS}" set image deploy/noppflow noppflow="${APP_IMAGE}" >"${out_file}" 2>&1; then
      rm -f "${out_file}"
      return 0
    fi

    if grep -q "timed out waiting for the condition" "${out_file}"; then
      echo "- warning: set image attempt ${i}/${attempts} timed out; retrying..."
      rm -f "${out_file}"
      sleep 2
      continue
    fi

    cat "${out_file}" >&2
    rm -f "${out_file}"
    return 1
  done

  echo "- warning: set image timed out after retries; validating deployment spec..."
  local current_image
  current_image="$(kubectl -n "${CONTROLLER_NS}" get deploy noppflow -o jsonpath='{.spec.template.spec.containers[?(@.name=="noppflow")].image}')"
  if [ "${current_image}" = "${APP_IMAGE}" ]; then
    echo "- image update confirmed after timeout"
    return 0
  fi

  echo "error: failed to update deployment image to ${APP_IMAGE}" >&2
  return 1
}

set_env_with_retry() {
  local mode="$1"
  local attempts=3
  local i
  local out_file
  local cmd=()

  if [ "${mode}" = "mysql" ]; then
    cmd=(kubectl -n "${CONTROLLER_NS}" set env deploy/noppflow DB_DRIVER=mysql)
  else
    cmd=(kubectl -n "${CONTROLLER_NS}" set env deploy/noppflow DB_DRIVER=sqlite3 DB_DSN-)
  fi

  for i in $(seq 1 "${attempts}"); do
    out_file="$(mktemp)"
    if "${cmd[@]}" >"${out_file}" 2>&1; then
      rm -f "${out_file}"
      return 0
    fi

    if grep -q "timed out waiting for the condition" "${out_file}"; then
      echo "- warning: set env attempt ${i}/${attempts} timed out; retrying..."
      rm -f "${out_file}"
      sleep 2
      continue
    fi

    cat "${out_file}" >&2
    rm -f "${out_file}"
    return 1
  done

  echo "- warning: set env timed out after retries; validating deployment spec..."
  local driver_value
  driver_value="$(kubectl -n "${CONTROLLER_NS}" get deploy noppflow -o jsonpath='{.spec.template.spec.containers[?(@.name=="noppflow")].env[?(@.name=="DB_DRIVER")].value}')"
  if [ "${mode}" = "mysql" ]; then
    if [ "${driver_value}" = "mysql" ]; then
      echo "- DB_DRIVER=mysql confirmed after timeout"
      return 0
    fi
  else
    local dsn_name
    dsn_name="$(kubectl -n "${CONTROLLER_NS}" get deploy noppflow -o jsonpath='{.spec.template.spec.containers[?(@.name=="noppflow")].env[?(@.name=="DB_DSN")].name}')"
    if [ "${driver_value}" = "sqlite3" ] && [ -z "${dsn_name}" ]; then
      echo "- sqlite env confirmed after timeout"
      return 0
    fi
  fi

  echo "error: failed to update deployment env for mode=${mode}" >&2
  return 1
}

rollout_status_with_fallback() {
  local deploy_name="$1"
  local timeout="${2:-300s}"
  local out_file
  out_file="$(mktemp)"

  if kubectl -n "${CONTROLLER_NS}" rollout status "deploy/${deploy_name}" --timeout="${timeout}" >"${out_file}" 2>&1; then
    rm -f "${out_file}"
    return 0
  fi

  if ! grep -q "timed out waiting for the condition" "${out_file}"; then
    cat "${out_file}" >&2
    rm -f "${out_file}"
    return 1
  fi
  rm -f "${out_file}"

  local desired updated ready available
  desired="$(kubectl -n "${CONTROLLER_NS}" get deploy "${deploy_name}" -o jsonpath='{.spec.replicas}')"
  updated="$(kubectl -n "${CONTROLLER_NS}" get deploy "${deploy_name}" -o jsonpath='{.status.updatedReplicas}')"
  ready="$(kubectl -n "${CONTROLLER_NS}" get deploy "${deploy_name}" -o jsonpath='{.status.readyReplicas}')"
  available="$(kubectl -n "${CONTROLLER_NS}" get deploy "${deploy_name}" -o jsonpath='{.status.availableReplicas}')"

  desired="${desired:-0}"
  updated="${updated:-0}"
  ready="${ready:-0}"
  available="${available:-0}"

  if [ "${desired}" -gt 0 ] && [ "${updated}" -ge "${desired}" ] && [ "${available}" -ge 1 ] && [ "${ready}" -ge 1 ]; then
    echo "- warning: rollout status timed out, but deployment is healthy (desired=${desired}, updated=${updated}, ready=${ready}, available=${available})"
    return 0
  fi

  echo "error: rollout timeout and deployment is not healthy yet (desired=${desired}, updated=${updated}, ready=${ready}, available=${available})" >&2
  echo "--- deployment describe (${CONTROLLER_NS}/${deploy_name}) ---" >&2
  kubectl -n "${CONTROLLER_NS}" describe deploy "${deploy_name}" >&2 || true
  echo "--- pods (${CONTROLLER_NS}, label app=${deploy_name}) ---" >&2
  kubectl -n "${CONTROLLER_NS}" get pods -l "app=${deploy_name}" -o wide >&2 || true
  echo "--- recent events (${CONTROLLER_NS}) ---" >&2
  kubectl -n "${CONTROLLER_NS}" get events --sort-by=.lastTimestamp | tail -n 30 >&2 || true
  return 1
}

seed_test_app() {
  echo "- Seeding test app (${TEST_APP_NAME})"
  COOKIE_FILE="$(mktemp)"
  kubectl -n "${CONTROLLER_NS}" port-forward svc/noppflow "${LOCAL_API_PORT}:80" >/tmp/noppflow-port-forward.log 2>&1 &
  PF_PID=$!

  for _ in $(seq 1 30); do
    if curl -fsS "http://127.0.0.1:${LOCAL_API_PORT}/health" >/dev/null 2>&1; then
      break
    fi
    sleep 1
  done
  if ! curl -fsS "http://127.0.0.1:${LOCAL_API_PORT}/health" >/dev/null 2>&1; then
    echo "error: noppflow API did not become reachable for seeding" >&2
    exit 1
  fi

  curl -fsS -c "${COOKIE_FILE}" -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"admin"}' \
    "http://127.0.0.1:${LOCAL_API_PORT}/api/auth/login" >/dev/null

  local keys_json
  keys_json="$(api_get "/api/ssh-keys")"
  if ! echo "${keys_json}" | grep -q "\"name\":\"${TEST_SSH_KEY_NAME}\""; then
    api_post_json "/api/ssh-keys" "{\"name\":\"${TEST_SSH_KEY_NAME}\",\"private_key\":\"dummy-private-key\"}" "201"
    echo "- created test SSH key: ${TEST_SSH_KEY_NAME}"
  else
    echo "- test SSH key already exists: ${TEST_SSH_KEY_NAME}"
  fi

  local apps_json
  apps_json="$(api_get "/api/apps")"
  if [ "${RESET_DATABASE}" = "true" ]; then
    local app_id
    while IFS= read -r app_id; do
      if [ -z "${app_id}" ]; then
        continue
      fi
      api_delete "/api/apps/${app_id}" "204"
      echo "- removed existing app id=${app_id}"
    done < <(extract_all_app_ids "${apps_json}")
    apps_json="[]"
  fi

  local payload
  payload="$(build_test_app_payload)"
  if ! echo "${apps_json}" | grep -q "\"name\":\"${TEST_APP_NAME}\""; then
    api_post_json "/api/apps" "${payload}" "201"
    echo "- created test app: ${TEST_APP_NAME}"
  else
    local app_id
    app_id="$(extract_app_id_by_name "${apps_json}" "${TEST_APP_NAME}")"
    if [ -z "${app_id}" ]; then
      echo "error: could not resolve id for existing test app ${TEST_APP_NAME}" >&2
      exit 1
    fi
    api_put_json "/api/apps/${app_id}" "${payload}" "200"
    echo "- updated test app: ${TEST_APP_NAME} (id=${app_id})"
  fi
}

echo "[1/10] Ensuring local registry container (${REGISTRY_NAME})"
if ! "${CONTAINER_CLI}" inspect -f '{{.State.Running}}' "${REGISTRY_NAME}" >/dev/null 2>&1; then
  "${CONTAINER_CLI}" run -d --restart=always -p "${REGISTRY_PORT}:5000" --name "${REGISTRY_NAME}" registry:2
else
  echo "- registry already running"
fi

echo "[2/10] Ensuring kind cluster (${CLUSTER_NAME})"
if ! kind get clusters | grep -qx "${CLUSTER_NAME}"; then
  create_kind_cluster
else
  echo "- cluster already exists"
fi

CONTROL_PLANE_CONTAINER="${CLUSTER_NAME}-control-plane"
control_plane_running="$("${CONTAINER_CLI}" inspect -f '{{.State.Running}}' "${CONTROL_PLANE_CONTAINER}" 2>/dev/null || echo "false")"
if [ "${control_plane_running}" != "true" ]; then
  echo "- detected stopped kind control-plane container (${CONTROL_PLANE_CONTAINER}); recreating cluster"
  kind delete cluster --name "${CLUSTER_NAME}" >/dev/null 2>&1 || true
  create_kind_cluster
fi

echo "- using kubectl context: ${KUBE_CONTEXT}"
if ! kubectl config get-contexts -o name | grep -qx "${KUBE_CONTEXT}"; then
  echo "- context ${KUBE_CONTEXT} not found; exporting kubeconfig from kind"
  kind export kubeconfig --name "${CLUSTER_NAME}" >/dev/null
fi
kubectl config use-context "${KUBE_CONTEXT}" >/dev/null
configure_kind_registry_http "${REGISTRY_ADDR}"
configure_kind_registry_http "${REGISTRY_IN_CLUSTER_ADDR}"
if [ "${ENABLE_INCLUSTER_REGISTRY}" = "true" ]; then
  configure_kind_registry_http "${INCLUSTER_REGISTRY_ADDR}"
fi

echo "[3/10] Connecting registry to kind network"
if [ "$("${CONTAINER_CLI}" inspect -f='{{json .NetworkSettings.Networks.kind}}' "${REGISTRY_NAME}")" = 'null' ]; then
  if ! "${CONTAINER_CLI}" network connect kind "${REGISTRY_NAME}"; then
    echo "error: failed to connect ${REGISTRY_NAME} to 'kind' network" >&2
    echo "hint: verify kind cluster is running and Podman/Docker can see network 'kind'" >&2
    exit 1
  fi
else
  echo "- registry already connected to kind network"
fi

if [ "${RESET_DATABASE}" = "true" ]; then
  echo "[4/10] Resetting local NoppFlow state (namespaces: ${CONTROLLER_NS}, ${APPS_NS})"
  kubectl delete ns "${CONTROLLER_NS}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true
  kubectl delete ns "${APPS_NS}" --ignore-not-found=true --wait=true >/dev/null 2>&1 || true
else
  echo "[4/10] Keeping existing local state (RESET_DATABASE=false)"
fi

echo "[5/10] Configuring local registry discovery in cluster"
cat <<MAP | kubectl apply -f -
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "${REGISTRY_ADDR}"
    help: "https://kind.sigs.k8s.io/docs/user/local-registry/"
MAP

echo "[6/10] Building and pushing images"
"${CONTAINER_CLI}" build -t "${APP_IMAGE}" -f Dockerfile .
push_image "${APP_IMAGE}"
"${CONTAINER_CLI}" build -t "${RUNNER_IMAGE}" -f Dockerfile.runner .
push_image "${RUNNER_IMAGE}"

echo "- loading images into kind nodes (fallback to avoid registry pull issues)"
load_kind_image_if_possible "${APP_IMAGE}"
load_kind_image_if_possible "${RUNNER_IMAGE}"

echo "[7/10] Creating namespaces"
kubectl get ns "${CONTROLLER_NS}" >/dev/null 2>&1 || kubectl create ns "${CONTROLLER_NS}"
kubectl get ns "${APPS_NS}" >/dev/null 2>&1 || kubectl create ns "${APPS_NS}"

if [ "${ENABLE_INCLUSTER_REGISTRY}" = "true" ]; then
  echo "- ensuring in-cluster registry service (${INCLUSTER_REGISTRY_NAME}) in namespace ${APPS_NS}"
  cat <<REGISTRY | kubectl -n "${APPS_NS}" apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ${INCLUSTER_REGISTRY_NAME}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: ${INCLUSTER_REGISTRY_NAME}
  template:
    metadata:
      labels:
        app: ${INCLUSTER_REGISTRY_NAME}
    spec:
      containers:
        - name: registry
          image: registry:2
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: 5000
              name: registry
---
apiVersion: v1
kind: Service
metadata:
  name: ${INCLUSTER_REGISTRY_NAME}
spec:
  selector:
    app: ${INCLUSTER_REGISTRY_NAME}
  ports:
    - name: registry
      port: 5000
      targetPort: registry
      protocol: TCP
REGISTRY
  kubectl -n "${APPS_NS}" rollout status deploy/"${INCLUSTER_REGISTRY_NAME}" --timeout=180s
fi

echo "[8/10] Preparing database mode (${DB_MODE})"
if [ "${DB_MODE}" = "mysql" ]; then
  kubectl -n "${CONTROLLER_NS}" apply -f k8s/mysql.kind.yaml
  kubectl -n "${CONTROLLER_NS}" create secret generic noppflow-db \
    --from-literal=dsn="noppflow:noppflow@tcp(mysql.${CONTROLLER_NS}.svc.cluster.local:3306)/noppflow?parseTime=true" \
    --dry-run=client -o yaml | kubectl apply -f -
  kubectl -n "${CONTROLLER_NS}" rollout status deploy/mysql --timeout=240s
else
  echo "- using sqlite for local setup (default)"
fi

echo "[9/10] Applying RBAC"
cat <<SA | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: noppflow-controller
  namespace: ${CONTROLLER_NS}
SA

cat <<CTRL | kubectl apply -f -
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: noppflow-controller
  namespace: ${APPS_NS}
rules:
  - apiGroups: ["batch"]
    resources: ["jobs"]
    verbs: ["get", "list", "watch", "create", "delete"]
  - apiGroups: [""]
    resources: ["pods", "pods/log"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "create", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: noppflow-controller
  namespace: ${APPS_NS}
subjects:
  - kind: ServiceAccount
    name: noppflow-controller
    namespace: ${CONTROLLER_NS}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: noppflow-controller
CTRL

cat <<RUNNER | kubectl apply -f -
apiVersion: v1
kind: ServiceAccount
metadata:
  name: noppflow-runner
  namespace: ${APPS_NS}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: noppflow-runner
  namespace: ${APPS_NS}
rules:
  - apiGroups: ["", "apps", "batch", "networking.k8s.io"]
    resources: ["pods", "services", "configmaps", "secrets", "deployments", "statefulsets", "jobs", "cronjobs", "ingresses"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: noppflow-runner
  namespace: ${APPS_NS}
subjects:
  - kind: ServiceAccount
    name: noppflow-runner
    namespace: ${APPS_NS}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: noppflow-runner
RUNNER

echo "[10/10] Deploying NoppFlow"
kubectl -n "${CONTROLLER_NS}" apply -f k8s/deployment.yaml
kubectl -n "${CONTROLLER_NS}" apply -f k8s/service.yaml
set_image_with_retry
if [ "${DB_MODE}" = "mysql" ]; then
  set_env_with_retry mysql
else
  set_env_with_retry sqlite
fi
rollout_status_with_fallback noppflow 300s

if [ "${SEED_TEST_APP}" = "true" ]; then
  seed_test_app
fi

echo
echo "Local kind environment is ready."
echo "- Cluster: ${CLUSTER_NAME}"
echo "- NoppFlow namespace: ${CONTROLLER_NS}"
echo "- Apps namespace: ${APPS_NS}"
echo "- App image: ${APP_IMAGE}"
echo "- Runner image: ${RUNNER_IMAGE}"
echo "- Runner image (job): ${RUNNER_IMAGE_JOB}"
echo "- Registry address: ${REGISTRY_ADDR}"
echo "- Runtime app registry: ${APP_RUNTIME_REGISTRY_ADDR}"
echo "- Runtime app registry (push from runner): ${APP_RUNTIME_REGISTRY_PUSH_ADDR}"
echo "- Runtime app registry (pull by kubelet): ${APP_RUNTIME_REGISTRY_PULL_ADDR}"
echo
echo "Access UI with:"
echo "kubectl -n ${CONTROLLER_NS} port-forward svc/noppflow 8080:80"
echo "Then open: http://localhost:8080 (admin/admin)"
echo
echo "When creating an app with k8s_deploy, use:"
echo "- k8s_namespace: ${APPS_NS}"
echo "- k8s_service_account: noppflow-runner"
echo "- k8s_runner_image: ${RUNNER_IMAGE_JOB}"
if [ "${SEED_TEST_APP}" = "true" ]; then
  echo
  echo "Seeded test app:"
  echo "- name: ${TEST_APP_NAME}"
  echo "- repo: ${TEST_APP_REPO}"
fi
