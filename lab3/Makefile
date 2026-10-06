CLUSTER_NAME := observability-demo

API_IMAGE    := shop-api
WORKER_IMAGE := shop-worker
TAG          := v1

.PHONY: help cluster-up cluster-down \
        install-operator \
        build build-api build-worker \
        image-load image-load-api image-load-worker \
        deploy deploy-shop \
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

build: build-api build-worker ## Собрать оба образа

build-api: ## Собрать образ shop-api
	docker build -t $(API_IMAGE):$(TAG) ./service/api

build-worker: ## Собрать образ shop-worker
	docker build -t $(WORKER_IMAGE):$(TAG) ./service/worker

install-operator: ## Установить CloudNativePG operator
	kubectl apply --server-side -f operator/cloudnative-pg.yaml
	kubectl wait --for=condition=Available \
	  deployment/cnpg-controller-manager \
	  -n cnpg-system \
	  --timeout=120s

image-load: image-load-api image-load-worker ## Загрузить оба образа в kind

image-load-api: build-api ## Загрузить shop-api в kind
	kind load docker-image $(API_IMAGE):$(TAG) --name $(CLUSTER_NAME)

image-load-worker: build-worker ## Загрузить shop-worker в kind
	kind load docker-image $(WORKER_IMAGE):$(TAG) --name $(CLUSTER_NAME)

deploy-shop: image-load ## Установить чарт shop через Helm
	helm upgrade --install shop ./charts/shop \
	  --namespace default \
	  --set api.image.repository=$(API_IMAGE) \
	  --set api.image.tag=$(TAG) \
	  --set worker.image.repository=$(WORKER_IMAGE) \
	  --set worker.image.tag=$(TAG) \
	  --wait \
	  --timeout 5m

deploy: cluster-up install-operator deploy-shop ## Развернуть shop

status: ## Показать состояние всего стенда
	@echo "=== Kind nodes ==="
	kubectl get nodes
	@echo
	@echo "=== Shop ==="
	kubectl get pods,svc
	@echo
	@echo "=== Monitoring ==="
	kubectl get pods,svc -n monitoring

port-forward-api: ## API: localhost:8080
	kubectl port-forward --address 0.0.0.0 svc/shop-api 8080:80

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