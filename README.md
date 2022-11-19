# Demo Project

This project was built during my studies on Kubernetes, Redis and Grafana Stack. It's a demo application written in Go with an HTTP server deployed on a local Kubernetes cluster.

## Tools

- [Kubernetes](https://kubernetes.io/)
- [Docker](https://www.docker.com/)
- [Kind](https://kind.sigs.k8s.io/)
- [Helm](https://helm.sh/)
- [Redis](https://redis.io/)
- [Grafana](https://grafana.com/)
- [Loki](https://grafana.com/oss/loki/)
- [Prometheus](https://prometheus.io/)                                 

## Setup

### Kubernetes Cluster and Local Registry

To run this project, you need to have a local k8s cluster, as well as a local registry where the Docker images will be stored. In this project I'm using the Kind tool to set up a local cluster with one node. You can follow this [tutorial](https://kind.sigs.k8s.io/docs/user/local-registry/) from the official documentation to create the cluster and configure the registry on your machine.

### .k8s folder 

This folder contains the k8s manifests needed to run the application on the cluster, which are: service, configmaps, deployment, hpa and pod. This folder is no longer being used in the project and is here for future reference only, as the project uses helm to create the k8s manifests.

### Namespaces

It is necessary to create two namespaces before applying changes to the cluster.

- **demo**: The namespace where the application and redis will be deployed.
- **monitoring**: The namespace where monitoring services will be deployed.

Creating the demo namespace:

```sh
kubectl create namespace demo
```

Creating the monitoring namespace:

```sh
kubectl create namespace monitoring
```

### Helm

Before deploying the application with Helm, it is necessary to install its dependencies first, which are: Redis, Grafana, Prometheus and Metrics Server.

***Obs**.: With the exception of the redis chart, all other dependencies installed with helm are using default settings. As the intention is not to delve into devops, I decided to follow the default settings that were already enough.

#### Installing [Redis](https://bitnami.com/stack/redis/helm)

```sh
make helm/install/redis
```

The release will be installed with the otpion `--set architecture=standalone` because it will deploy a standalone Redis StatefulSet. Only one service will be exposed that points to the master, where read-write operations can be performed.

#### Installing [Grafana](https://github.com/grafana/helm-charts)

```sh
make helm/install/grafana
```

#### Installing [Grafana Loki](https://bitnami.com/stack/grafana-loki/helm)

```sh
make helm/install/loki
```

Follow this [tutorial](https://grafana.com/docs/grafana/latest/datasources/add-a-data-source/) on how to add the loki data source to grafana. The only important field is the url of the grafana loki gateway service, which is: `http://grafana-loki-gateway.monitoring:80`.

#### Installing [Prometheus](https://github.com/prometheus-community/helm-charts)

```sh
make helm/install/prometheus
```

In order to get prometheus to scrape pods, It was necessary to add the following annotations to the the pods inside the values.yaml file:

```
annotations:
    prometheus.io/scrape: "true"
    prometheus.io/path: /metrics
    prometheus.io/port: "http"
```

Follow this [tutorial](https://grafana.com/docs/grafana/latest/datasources/add-a-data-source/) on how to add the prometheus data source to grafana. The only important field is the url of the prometheus server service, which is: `http://prometheus-server.monitoring:80`.

#### Installing [Metrics Server](https://artifacthub.io/packages/helm/metrics-server/metrics-server)

It is necessary to have a Metrics Server deployed to allow horizontal scaling with HPA. I'm using the official k8s [Metrics Server](https://github.com/kubernetes-sigs/metrics-server) which has a horizontal scaling limitation based only on CPU and memory resources.

```sh
make helm/install/metrics-server
```

It is necessary to disable certificate validation to deploy locally. To do so, just pass a flag in the arguments of the container's template as follows:

```sh
kubectl edit -n kube-system deployments.apps metrics-server
```

Add `--kubelet-insecure-tls` to the end of the container's argument list. Example:

```
...
spec:
      containers:
      - args:
        - --secure-port=4443
        - --cert-dir=/tmp
        - --kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname
        - --kubelet-use-node-status-port
        - --metric-resolution=15s
        - --kubelet-insecure-tls (NEW LINE!)
...
```

#### Installing application

```sh
make helm/install
```

## Testing the application

### Port Forward

```sh
make demo/port-forward
```

```sh
make demo/test
```

### Grafana

Get the admin password to access the Grafana dashboard:

```sh
make grafana/admin-password
```

Port Forward:

```sh
make grafana/port-forward
```

Open the [Dashboard](http://localhost:3000).

### HPA

Increase the load on the pod by calling `make demo/load-generator id=1` several times passing diferent ids. Watch HPA to scale up the number of pods. After a while, stop all the load generatos and watch the HPA to scale down the number of pods.

```sh
kubectl get hpa --watch
```
