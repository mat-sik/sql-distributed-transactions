# Apply all objects

```bash
 minikube kubectl -- apply -f k8s
```

# Delete all objects

```bash
 minikube kubectl -- delete -f k8s
```

# Create directories for PVs

```bash
minikube ssh

sudo rm -rf /mnt/data &&

sudo mkdir -p /mnt/data/server-psql &&
sudo chown 999:999 /mnt/data/server-psql &&

sudo mkdir -p /mnt/data/prometheus &&
sudo chown 65534:65534 /mnt/data/prometheus &&

sudo mkdir -p /mnt/data/minio &&

sudo mkdir -p /mnt/data/tempo &&
sudo chown 10001:10001 /mnt/data/tempo &&

sudo mkdir -p /mnt/data/mimir &&

sudo mkdir -p /mnt/data/loki &&
sudo chown 10001:10001 /mnt/data/loki &&

sudo mkdir -p /mnt/data/alloy &&
sudo chown 473:473 /mnt/data/alloy &&

sudo chmod 700 /mnt/data/* &&

exit
```

# PostgreSQL accessible on localhost

```bash
minikube kubectl -- port-forward service/server-psql 5432:5432
```

# Grafana accessible on localhost

```bash
minikube kubectl -- port-forward service/grafana 3000:3000
```

# Minio accessible on localhost

```bash
minikube kubectl -- port-forward service/minio 9001:9001 
```

# Create images for minikube from host docker

```bash
eval $(minikube docker-env) 

for img in server dummy client; do
  docker build -t sql-distributed-transactions-$img ./$img
done

eval $(minikube docker-env -u)
```

# Provide images from host to minikube

```bash
for img in server dummy client; do
  docker build -t sql-distributed-transactions-$img ./$img
done
```

```bash
for img in server dummy client; do
  minikube image load sql-distributed-transactions-$img
done
```

# Create configmaps with config files

```bash
CONFIGMAPS=(
  "otel-collector"
  "prometheus"
  "tempo"
  "mimir"
  "loki"
  "grafana"
  "alloy"
)

for name in "${CONFIGMAPS[@]}"; do
  cm="${name}-config-yaml-configmap"
  minikube kubectl -- delete configmap "$cm" --ignore-not-found
done
```

```bash
minikube kubectl -- create configmap otel-collector-config-yaml-configmap \
  --from-file=otel-collector.yaml=./otel-collector.yaml
  
minikube kubectl -- create configmap prometheus-config-yaml-configmap \
  --from-file=prometheus.yaml=./prometheus.yaml
  
minikube kubectl -- create configmap tempo-config-yaml-configmap \
  --from-file=tempo.yaml=./tempo.yaml
  
minikube kubectl -- create configmap mimir-config-yaml-configmap \
  --from-file=mimir.yaml=./mimir.yaml
  
minikube kubectl -- create configmap loki-config-yaml-configmap \
  --from-file=loki.yaml=./loki.yaml
  
minikube kubectl -- create configmap grafana-config-yaml-configmap \
  --from-file=datasources.yaml=./grafana-datasources.yaml
  
minikube kubectl -- create configmap alloy-config-yaml-configmap \
  --from-file=config.alloy=./config-k8s.alloy
```
