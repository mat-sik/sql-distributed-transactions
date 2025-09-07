# PostgreSQL Local PersistentVolume Setup in Minikube

```bash
minikube start
```

# Create directories for PVs

```bash
minikube ssh

sudo mkdir -p /mnt/data/server-psql &&
sudo chown 999:999 /mnt/data/server-psql &&
sudo chmod 700 /mnt/data/server-psql &&

sudo mkdir -p /mnt/data/prometheus &&
sudo chown 65534:65534 /mnt/data/prometheus &&
sudo chmod 700 /mnt/data/prometheus &&

sudo mkdir -p /mnt/data/minio &&
sudo chmod 700 /mnt/data/minio &&

sudo mkdir -p /mnt/data/tempo &&
sudo chown 10001:10001 /mnt/data/tempo &&
sudo chmod 700 /mnt/data/tempo &&

sudo mkdir -p /mnt/data/mimir &&
sudo chmod 700 /mnt/data/mimir &&

sudo mkdir -p /mnt/data/loki &&
sudo chown 10001:10001 /mnt/data/loki &&
sudo chmod 700 /mnt/data/loki &&

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
```

```bash
eval $(minikube docker-env -u)
```

# Provide images from host to minikube

```bash
docker build -t sql-distributed-transactions-server ./server &&
docker build -t sql-distributed-transactions-dummy ./dummy &&
docker build -t sql-distributed-transactions-client ./client
```

```bash
minikube image rm sql-distributed-transactions-server &&
minikube image rm sql-distributed-transactions-dummy &&
minikube image rm sql-distributed-transactions-client
```

```bash
minikube image load sql-distributed-transactions-server &&
minikube image load sql-distributed-transactions-dummy &&
minikube image load sql-distributed-transactions-client
```

# Create configmaps with config files

```bash
minikube kubectl -- delete configmap otel-collector-config-yaml-configmap &&
minikube kubectl -- delete configmap prometheus-config-yaml-configmap &&
minikube kubectl -- delete configmap tempo-config-yaml-configmap &&
minikube kubectl -- delete configmap mimir-config-yaml-configmap &&
minikube kubectl -- delete configmap loki-config-yaml-configmap &&
minikube kubectl -- delete configmap grafana-config-yaml-configmap &&

minikube kubectl -- create configmap otel-collector-config-yaml-configmap \
  --from-file=config.yaml=./otel-collector.yaml &&
  
minikube kubectl -- create configmap prometheus-config-yaml-configmap \
  --from-file=prometheus.yaml=./prometheus.yaml &&
  
minikube kubectl -- create configmap tempo-config-yaml-configmap \
  --from-file=tempo.yaml=./tempo.yaml &&
  
minikube kubectl -- create configmap mimir-config-yaml-configmap \
  --from-file=mimir.yaml=./mimir.yaml &&
  
minikube kubectl -- create configmap loki-config-yaml-configmap \
  --from-file=loki.yaml=./loki.yaml &&
  
minikube kubectl -- create configmap grafana-config-yaml-configmap \
  --from-file=datasources.yaml=./grafana-datasources.yaml
```
