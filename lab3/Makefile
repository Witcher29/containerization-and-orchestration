CLUSTER_NAME := observability-demo

API_IMAGE    := shop-api
WORKER_IMAGE := shop-worker
TAG          := v1

.PHONY: help cluster-up cluster-down \
        install-kyverno apply-policies \
        build build-api build-worker \
        image-load image-load-api image-load-worker \
        install-monitoring \
        deploy deploy-shop \
        port-forward-api port-forward-grafana port-forward-prometheus \
        port-forward-jaeger port-forward-alertmanager \
        status clean

help: ## Показать доступные команды
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-28s\033[0m %s\n", $$1, $$2}'

# ---------- Кластер ----------
cluster-up: ## Создать Kind-кластер
	./cluster/create.sh

cluster-down: ## Удалить Kind-кластер
	./cluster/delete.sh

# ---------- Политики ----------
install-kyverno: ## Установить Kyverno
	helm repo add kyverno https://kyverno.github.io/kyverno/ 2>/dev/null || true
	helm repo update
	helm upgrade --install kyverno kyverno/kyverno \
	  --namespace kyverno \
	  --create-namespace \
	  --wait \
	  --timeout 10m

apply-policies: ## Применить 5 политик из policies/
	kubectl apply -f policies/

# ---------- Сборка образов ----------
build: build-api build-worker ## Собрать оба образа

build-api: ## Собрать образ shop-api
	docker build -t $(API_IMAGE):$(TAG) ./service/api

build-worker: ## Собрать образ shop-worker
	docker build -t $(WORKER_IMAGE):$(TAG) ./service/worker

image-load: image-load-api image-load-worker ## Загрузить оба образа в kind

image-load-api: build-api ## Загрузить shop-api в kind
	kind load docker-image $(API_IMAGE):$(TAG) --name $(CLUSTER_NAME)

image-load-worker: build-worker ## Загрузить shop-worker в kind
	kind load docker-image $(WORKER_IMAGE):$(TAG) --name $(CLUSTER_NAME)

# ---------- Мониторинг (часть 5) ----------
install-monitoring: ## Установить стек мониторинга из lab2
	helm dependency update ./charts/observability
	helm upgrade --install observability ./charts/observability \
	  --namespace monitoring \
	  --create-namespace \
	  --wait \
	  --timeout 15m

# ---------- Shop ----------
deploy-shop: image-load ## Установить чарт shop
	helm upgrade --install shop ./charts/shop \
	  --namespace default \
	  --set api.image.repository=$(API_IMAGE) \
	  --set api.image.tag=$(TAG) \
	  --set worker.image.repository=$(WORKER_IMAGE) \
	  --set worker.image.tag=$(TAG) \
	  --wait \
	  --timeout 5m

# ---------- Полный деплой ----------
deploy: cluster-up install-kyverno apply-policies deploy-shop ## Развернуть shop

deploy-all: cluster-up install-kyverno apply-policies install-monitoring deploy-shop ## Всё сразу (включая мониторинг)

# ---------- Диагностика ----------
status: ## Показать состояние стенда
	@echo "=== Nodes ==="
	kubectl get nodes
	@echo
	@echo "=== Kyverno ==="
	kubectl get pods -n kyverno
	kubectl get validatingpolicies
	@echo
	@echo "=== Shop ==="
	kubectl get pods,svc
	@echo
	@echo "=== Monitoring ==="
	kubectl get pods,svc -n monitoring

# ---------- Проброс портов ----------
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

# ---------- Уборка ----------
clean: cluster-down ## Удалить кластер
	rm -f shop-api shop-worker