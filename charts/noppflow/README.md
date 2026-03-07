# NoppFlow Helm Chart

## Install

```bash
helm upgrade --install noppflow ./charts/noppflow \
  --namespace noppflow \
  --create-namespace
```

Default values run with SQLite (`DB_DRIVER=sqlite3`).

## kind/local profile

Use the bundled kind values file:

```bash
helm upgrade --install noppflow ./charts/noppflow \
  --namespace noppflow \
  --create-namespace \
  -f ./charts/noppflow/values-kind.yaml
```

If your locally built image uses a custom tag:

```bash
helm upgrade --install noppflow ./charts/noppflow \
  --namespace noppflow \
  --create-namespace \
  -f ./charts/noppflow/values-kind.yaml \
  --set image.tag=20260306153000
```

## MySQL example

Create DB secret first:

```bash
kubectl -n noppflow create secret generic noppflow-db \
  --from-literal=dsn='user:password@tcp(mysql-host:3306)/noppflow?parseTime=true'
```

Install/upgrade:

```bash
helm upgrade --install noppflow ./charts/noppflow \
  --namespace noppflow \
  --create-namespace \
  --set env.db.driver=mysql \
  --set env.db.existingSecret=noppflow-db
```

## RBAC scope

By default, the chart creates namespace-scoped RBAC in `apps` namespace:

```bash
--set rbac.scope=namespace --set rbac.targetNamespace=apps
```

For broader access, use cluster scope:

```bash
--set rbac.scope=cluster
```

## Access

```bash
kubectl -n noppflow port-forward svc/noppflow 8080:80
```

Open `http://127.0.0.1:8080` (`admin/admin` by default).
