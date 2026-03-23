---
name: scaffold-k8s
description: Generates the Kubernetes manifests (deployment, service, configmap, secret.example) for a JobRadar gRPC service. Invoke when creating k8s manifests for a new service. Argument: service name (e.g. rag-service).
user-invocable: false
---

Create the following four files for `$ARGUMENTS`. Replace `$PORT` with the gRPC port number and `$SECRETS` with the service-specific secret env vars.

## k8s/manifests/$ARGUMENTS/deployment.yaml

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $ARGUMENTS
  namespace: jobradar
  labels:
    app: $ARGUMENTS
    app.kubernetes.io/name: $ARGUMENTS
    app.kubernetes.io/part-of: jobradar
spec:
  replicas: 1
  selector:
    matchLabels:
      app: $ARGUMENTS
  template:
    metadata:
      labels:
        app: $ARGUMENTS
        app.kubernetes.io/name: $ARGUMENTS
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9090"
    spec:
      serviceAccountName: jobradar
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: $ARGUMENTS
          image: ghcr.io/pgrau/jobradar/$ARGUMENTS:latest
          imagePullPolicy: IfNotPresent
          ports:
            - name: grpc
              containerPort: $PORT
              protocol: TCP
          envFrom:
            - configMapRef:
                name: $ARGUMENTS-config
          env: [] # TODO: add secretKeyRef entries for sensitive values
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop: ["ALL"]
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 200m
              memory: 128Mi
          livenessProbe:
            grpc:
              port: $PORT
            initialDelaySeconds: 10
            periodSeconds: 15
            failureThreshold: 3
          readinessProbe:
            grpc:
              port: $PORT
            initialDelaySeconds: 5
            periodSeconds: 10
            failureThreshold: 3
      terminationGracePeriodSeconds: 30
```

## k8s/manifests/$ARGUMENTS/service.yaml

```yaml
apiVersion: v1
kind: Service
metadata:
  name: $ARGUMENTS
  namespace: jobradar
  labels:
    app: $ARGUMENTS
    app.kubernetes.io/name: $ARGUMENTS
    app.kubernetes.io/part-of: jobradar
spec:
  type: ClusterIP
  selector:
    app: $ARGUMENTS
  ports:
    - name: grpc
      port: $PORT
      targetPort: grpc
      protocol: TCP
```

## k8s/manifests/$ARGUMENTS/configmap.yaml

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: $ARGUMENTS-config
  namespace: jobradar
  labels:
    app: $ARGUMENTS
    app.kubernetes.io/name: $ARGUMENTS
    app.kubernetes.io/part-of: jobradar
data:
  ENV: "local"
  # TODO: add service-specific non-sensitive env vars
  OTEL_EXPORTER_OTLP_ENDPOINT: "alloy-otlp:4317"
  OTEL_SERVICE_NAMESPACE: "jobradar"
```

## k8s/manifests/$ARGUMENTS/secret.yaml.example

```yaml
# Copy to secret.yaml and fill in real values before deploying.
# secret.yaml is gitignored — never commit real credentials.
apiVersion: v1
kind: Secret
metadata:
  name: $ARGUMENTS-secret
  namespace: jobradar
type: Opaque
stringData: {}
  # TODO: add service-specific secret keys, e.g.:
  # postgres-password: "REPLACE_ME"
```

After generating, fill in the TODO sections with the actual env vars from the service's config struct.
