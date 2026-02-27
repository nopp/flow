# Kubernetes manifests for NoppFlow

## Prerequisites

- Build and push the container image (e.g. `noppflow:latest`) to your registry, or use a local image with `imagePullPolicy: IfNotPresent`.
- MySQL running (in-cluster or external). Create the database and a user for NoppFlow.

## 1. Create the database secret

Create the secret with the MySQL DSN (replace with your values):

```bash
kubectl create secret generic noppflow-db \
  --from-literal=dsn='user:password@tcp(mysql-host:3306)/noppflow?parseTime=true'
```

See `secret.example.yaml` for more options.

## 2. Deploy

```bash
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
