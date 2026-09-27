CLUSTER_NAME := observability-demo
IMAGE        := api-service
TAG          := latest

.PHONY: help cluster-up cluster-down build image-load \
        deploy deploy-api deploy-observability \
        port-forward-api port-forward-grafana port-forward-prometheus \
        port-forward-jaeger port-forward-alertmanager \
        status clean

help: ## Показать доступные команды
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}'

cluster-up: ## Создать Kind-кластер
	./cluster/create.sh

cluster-down: ## Удалить Kind-кластер
	./cluster/delete.sh

build: ## Собрать Docker-образ API
	docker build -t $(IMAGE):$(TAG) ./service

image-load: build ## Собрать API и загрузить image в Kind
	kind load docker-image $(IMAGE):$(TAG) --name $(CLUSTER_NAME)

deploy-api: image-load ## Установить API через Helm
	helm upgrade --install api-service ./charts/api-service \
	  --namespace default \
	  --set image.repository=$(IMAGE) \
	  --set image.tag=$(TAG) \
	  --wait \
	  --timeout 5m

deploy-observability: ## Установить observability stack через Helm
	helm dependency update ./charts/observability
	helm upgrade --install observability ./charts/observability \
	  --namespace monitoring \
	  --create-namespace \
	  --wait \
	  --timeout 10m

deploy: cluster-up deploy-api deploy-observability ## Полностью развернуть лабораторию

status: ## Показать состояние всего стенда
	@echo "=== Kind nodes ==="
	kubectl get nodes
	@echo
	@echo "=== API ==="
	kubectl get pods,svc
	@echo
	@echo "=== Monitoring ==="
	kubectl get pods,svc -n monitoring

port-forward-api: ## API: localhost:8080
	kubectl port-forward --address 0.0.0.0 svc/api-service-api-service 8080:80

port-forward-grafana: ## Grafana: localhost:3000
	kubectl port-forward --address 0.0.0.0 -n monitoring svc/observability-grafana 3000:80

port-forward-prometheus: ## Prometheus: localhost:9090
	kubectl port-forward --address 0.0.0.0 -n monitoring svc/observability-kube-prometh-prometheus 9090:9090

port-forward-jaeger: ## Jaeger: localhost:16686
	kubectl port-forward --address 0.0.0.0 -n monitoring svc/observability-jaeger 16686:16686

port-forward-alertmanager: ## Alertmanager: localhost:9093
	kubectl port-forward --address 0.0.0.0 -n monitoring svc/observability-kube-prometh-alertmanager 9093:9093

clean: ## Удалить весь Kind-кластер
	./cluster/delete.sh