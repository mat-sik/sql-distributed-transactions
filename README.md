# PostgreSQL Local PersistentVolume Setup in Minikube

```bash
minikube start
```

```bash
sudo mkdir -p /mnt/data/server-psql
```

```bash
sudo chown 999:999 /mnt/data/server-psql
sudo chmod 700 /mnt/data/server-psql
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