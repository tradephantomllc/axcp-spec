# Enterprise Node Helm Chart

This Helm chart deploys the **AXCP Enterprise Node** to a Kubernetes cluster (including K3s).

## Prerequisites

* Kubernetes v1.22+
* Helm v3.8+
* A valid container registry pull secret (if the image is private)

## Installing the Chart

```bash
helm repo add axcp https://tradephantom.github.io/helm-charts
helm install my-ent axcp/enterprise-node \
  --set jwtSecret="<jwt-secret>" \
  --set tlsCert="<base64-pem-cert>" \
  --set tlsKey="<base64-pem-key>"
```

> You **must** provide `jwtSecret`, `tlsCert`, and `tlsKey`; otherwise, the node will refuse to start.

## Configuration

The following table lists the configurable parameters exposed via `values.yaml`.

| Parameter | Description | Default |
|-----------|-------------|---------|
| `jwtSecret` | Secret used to sign/verify JWT tokens | `""` |
| `tlsCert` | Base64-encoded PEM certificate for mTLS | `""` |
| `tlsKey` | Base64-encoded PEM private key for mTLS | `""` |
| `piiSchema` | JSON PII schema path or inline value | `""` |
| `image.repository` | Container image repository | `ghcr.io/tradephantom/axcp-enterprise` |
| `image.tag` | Container image tag | `v0.3-edge-beta` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |
| `service.type` | Kubernetes Service type | `ClusterIP` |
| `service.port` | Service port | `7143` |
| `resources.*` | CPU/Memory requests & limits | See `values.yaml` |

## Testing Locally

```bash
cd charts/enterprise-node
helm lint .
helm template . \
  --set jwtSecret=mysecret \
  --set tlsCert=abc \
  --set tlsKey=def
```

## Uninstalling

```bash
helm uninstall my-ent
```

---

© TradePhantom. Licensed under the Enterprise license.
