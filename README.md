# PostgreSQL Local PersistentVolume Setup in Minikube

```bash
minikube start
```

```bash
sudo mkdir -p /mnt/data/server-psql
sudo chown 999:999 /mnt/data/server-psql
sudo chmod 700 /mnt/data/server-psql
```

# Prometheus Local PersistentVolume Setup in Minikube

```bash
sudo mkdir -p /mnt/data/prometheus
sudo chown 65534:65534 /mnt/data/prometheus
sudo chmod 700 /mnt/data/prometheus
```

# Minio Local PersistentVolume Setup in Minikube

```bash
sudo mkdir -p /mnt/data/minio
sudo chmod 700 /mnt/data/minio
```

# PostgreSQL accessible on localhost

```bash
minikube kubectl -- port-forward service/server-psql 5432:5432
```

# Create images for minikube from host docker

```bash
eval $(minikube docker-env)
```

```bash
eval $(minikube docker-env -u)
```

# Create configmap with a config file for otel collector

```bash
minikube kubectl -- create configmap otel-collector-config-yaml-configmap \
  --from-file=config.yaml=./otel-collector.yaml
```

# Create configmap with a config file for prometheus

```bash
minikube kubectl -- create configmap prometheus-config-yaml-configmap \
  --from-file=prometheus.yaml=./prometheus.yaml
```
