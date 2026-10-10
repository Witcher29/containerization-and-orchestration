CLUSTER_NAME := observability-demo

API_IMAGE := shop-api
WORKER_IMAGE := shop-worker
TAG := v1

OBSERVABILITY_CHART := ./charts/observability
SHOP_CHART := ./charts/shop

.PHONY: help cluster-up cluster-down \
	build-api build-worker build \
	install-operator load-images \
	observability-deps deploy-observability \
	deploy-shop deploy status clean \
	port-forward-api port-forward-grafana \
	port-forward-prometheus port-forward-alertmanager \
	port-forward-jaeger

help: ## Показать список доступных команд
	@grep -E '^[a-zA-Z0-9_-]+:.##' $(MAKEFILE_LIST) |
	awk 'BEGIN {FS = ":.## "}; {printf "  %-28s %s\n", $$1, $$2}'

cluster-up: ## Создать Kind-кластер
	./scripts/cluster-up.sh

cluster-down: ## Удалить Kind-кластер
	./scripts/cluster-down.sh

build-api: ## Собрать образ API
	docker build -t $(API_IMAGE):$(TAG) ./services/api

build-worker: ## Собрать образ worker
	docker build -t $(WORKER_IMAGE):$(TAG) ./services/worker

build: build-api build-worker ## Собрать образы API и worker

install-operator: ## Установить CloudNativePG operator
	helm repo add cloudnative-pg https://cloudnative-pg.github.io/charts
	helm repo update
	helm upgrade --install cnpg cloudnative-pg/cloudnative-pg
	--namespace cnpg-system
	--create-namespace
	--wait
	--timeout 5m

load-images: build ## Загрузить образы в Kind
	kind load docker-image $(API_IMAGE):$(TAG) --name $(CLUSTER_NAME)
	kind load docker-image $(WORKER_IMAGE):$(TAG) --name $(CLUSTER_NAME)

observability-deps: ## Скачать зависимости Helm-чарта observability
	helm dependency update $(OBSERVABILITY_CHART)

deploy-observability: observability-deps ## Установить стек observability
	helm upgrade --install observability $(OBSERVABILITY_CHART)
	--namespace monitoring
	--create-namespace
	--wait
	--timeout 10m

deploy-shop: load-images ## Установить приложение
	helm upgrade --install shop $(SHOP_CHART)
	--namespace default
	--set api.image.repository=$(API_IMAGE)
	--set api.image.tag=$(TAG)
	--set worker.image.repository=$(WORKER_IMAGE)
	--set worker.image.tag=$(TAG)
	--wait
	--timeout 5m

deploy: ## Развернуть кластер, observability и приложение
	$(MAKE) cluster-up
	$(MAKE) install-operator
	$(MAKE) deploy-observability
	$(MAKE) deploy-shop

status: ## Показать состояние ресурсов Kubernetes
	kubectl get nodes
	kubectl get pods -A
	kubectl get svc -A

port-forward-api: ## Открыть API на порту 8080
	kubectl port-forward --address 0.0.0.0 svc/shop-api 8080:80 -n default

port-forward-grafana: ## Открыть Grafana на порту 3000
	kubectl port-forward --address 0.0.0.0 svc/observability-grafana 3000:80 -n monitoring

port-forward-prometheus: ## Открыть Prometheus на порту 9090
	kubectl port-forward --address 0.0.0.0 svc/observability-kube-prometheus-prometheus 9090:9090 -n monitoring

port-forward-alertmanager: ## Открыть Alertmanager на порту 9093
	kubectl port-forward --address 0.0.0.0 svc/observability-kube-prometheus-alertmanager 9093:9093 -n monitoring

port-forward-jaeger: ## Открыть Jaeger UI на порту 16686
	kubectl port-forward --address 0.0.0.0 svc/observability-jaeger 16686:16686 -n monitoring

clean: ## Удалить Helm-релизы приложения и observability
	helm uninstall shop -n default || true helm uninstall observability -n monitoring || true