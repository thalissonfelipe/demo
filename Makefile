########## Build ##########

.PHONY: build
build:
	@echo "==> Building docker image and pushing it to local register..."
	@docker build -t demo:latest .
	@docker tag demo:latest localhost:5000/demo:latest
	@docker push localhost:5000/demo:latest

########## Helm Install ##########

.PHONY: helm/install/redis
helm/install/redis:
	@echo "==> Installing redis helm release..."
	@helm repo add bitnami https://charts.bitnami.com/bitnami
	@helm repo update
	@helm install -n demo redis bitnami/redis --set architecture=standalone

.PHONY: helm/install/grafana
helm/install/grafana:
	@echo "==> Installing grafana helm release..."
	@helm repo add grafana https://grafana.github.io/helm-charts
	@helm repo update
	@helm install -n monitoring grafana grafana/grafana

.PHONY: helm/install/loki
helm/install/loki:
	@echo "==> Installing loki helm release..."
	@helm repo add bitnami https://charts.bitnami.com/bitnami
	@helm repo update
	@helm install -n monitoring grafana-loki bitnami/grafana-loki

.PHONY: helm/install/prometheus
helm/install/prometheus:
	@echo "==> Installing prometheus helm release..."
	@helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	@helm repo update
	@helm install -n monitoring prometheus prometheus-community/prometheus

.PHONY: helm/install/metrics-server
helm/install/metrics-server:
	@echo "==> Installing metrics-server helm release..."
	@helm repo add metrics-server https://kubernetes-sigs.github.io/metrics-server/
	@helm repo update
	@helm install -n kube-system metrics-server metrics-server/metrics-server

########## Helm Uninstall ##########

.PHONY: helm/uninstall/redis
helm/uninstall/redis:
	@echo "==> Uninstalling redis helm release..."
	@helm uninstall -n demo redis

.PHONY: helm/uninstall/grafana
helm/uninstall/grafana:
	@echo "==> Uninstalling grafana helm release..."
	@helm uninstall -n monitoring grafana

.PHONY: helm/uninstall/loki
helm/uninstall/loki:
	@echo "==> Uninstalling loki helm release..."
	@helm uninstall -n monitoring loki

.PHONY: helm/uninstall/prometheus
helm/uninstall/prometheus:
	@echo "==> Uninstalling prometheus helm release..."
	@helm uninstall -n monitoring prometheus

.PHONY: helm/uninstall/metrics-server
helm/uninstall/metrics-server:
	@echo "==> Uninstalling metrics-server helm release..."
	@helm uninstall -n kube-system metrics-server

########## Grafana ##########

.PHONY: grafana/port-forward
grafana/port-forward:
	@echo "==> Grafana port forwarding on port 3000..."
	@kubectl port-forward -n monitoring services/grafana 3000:80

.PHONY: grafana/admin-password
grafana/admin-password:
	@echo "==> Getting Grafana admin password..."
	@kubectl get secret -n monitoring grafana -o jsonpath="{.data.admin-password}" | base64 --decode

########## Demo Application ##########

.PHONY: demo/upgrade
demo/upgrade:
	@echo "==> Upgrading demo helm values..."
	@helm upgrade -n demo -f demo/values.yaml demo ./demo

.PHONY: demo/install
demo/install:
	@echo "==> Installing demo helm release..."
	@helm install -n demo demo ./demo

.PHONY: demo/uninstall
demo/uninstall:
	@echo "==> Uninstalling demo helm release..."
	@helm uninstall -n demo demo

.PHONY: demo/port-forward
demo/port-forward:
	@echo "==> Demo application port forwarding on port 3001..."
	@kubectl port-forward -n demo services/demo 3001:80

.PHONY: demo/test
demo/test:
	@echo "==> Testing the demo application..."
	@curl http://localhost:3001/hello

.PHONY: demo/load-generator
demo/load-generator:
	@echo "==> Running load generator to increase load into the pod..."
	@kubectl run -i --tty load-generator-$(id) --rm --image=busybox:1.28 --restart=Never \
		-- /bin/sh -c "while sleep 0.01; do wget -q -O- http://demo/hello; done"
