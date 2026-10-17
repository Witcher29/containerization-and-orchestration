CLUSTER_NAME := observability-demo

API_IMAGE    := shop-api
WORKER_IMAGE := shop-worker
BATCH_IMAGE  := shop-batch
TAG          := v1

.PHONY: help cluster-up cluster-down \
        install-kyverno apply-policies install-monitoring install-operator \
        build build-api build-worker build-batch \
        image-load image-load-api image-load-worker image-load-batch \
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

# ---------- Политики (часть 1 lab3, переиспользуются в lab4) ----------
install-kyverno: ## Установить Kyverno
	helm repo add kyverno https://kyverno.github.io/kyverno/ 2>/dev/null || true
	helm repo update
	helm upgrade --install kyverno kyverno/kyverno \
	  --namespace kyverno --create-namespace --wait --timeout 10m

apply-policies: ## Применить политики
	kubectl apply -f policies/

# ---------- Мониторинг (Lab 2, переиспользуется в lab4) ----------
install-monitoring: ## Установить стек мониторинга
	helm dependency update ./charts/observability
	helm upgrade --install observability ./charts/observability \
	  --namespace monitoring --create-namespace --wait --timeout 15m

# ---------- Оператор Postgres ----------
install-operator: ## Установить CloudNativePG operator
	kubectl apply --server-side -f operator/cloudnative-pg.yaml
	kubectl wait --for=condition=Available \
	  deployment/cnpg-controller-manager -n cnpg-system --timeout=120s

# ---------- Сборка образов ----------
build: build-api build-worker build-batch

build-api: ## Собрать shop-api
	docker build -t $(API_IMAGE):$(TAG) ./service/api

build-worker: ## Собрать shop-worker
	docker build -t $(WORKER_IMAGE):$(TAG) ./service/worker

build-batch: ## Собрать shop-batch
	docker build -t $(BATCH_IMAGE):$(TAG) ./service/batch

image-load: image-load-api image-load-worker image-load-batch

image-load-api: build-api
	kind load docker-image $(API_IMAGE):$(TAG) --name $(CLUSTER_NAME)

image-load-worker: build-worker
	kind load docker-image $(WORKER_IMAGE):$(TAG) --name $(CLUSTER_NAME)

image-load-batch: build-batch
	kind load docker-image $(BATCH_IMAGE):$(TAG) --name $(CLUSTER_NAME)

# ---------- Shop ----------
deploy-shop: image-load ## Установить чарт shop
	helm upgrade --install shop ./charts/shop \
	  --namespace default \
	  --set api.image.repository=$(API_IMAGE) \
	  --set api.image.tag=$(TAG) \
	  --set worker.image.repository=$(WORKER_IMAGE) \
	  --set worker.image.tag=$(TAG) \
	  --set batch.image.repository=$(BATCH_IMAGE) \
	  --set batch.image.tag=$(TAG) \
	  --wait --timeout 5m

deploy: cluster-up install-kyverno apply-policies install-operator deploy-shop

# ---------- Диагностика ----------
status:
	@echo "=== Nodes ==="
	kubectl get nodes
	@echo
	@echo "=== Shop ==="
	kubectl get pods,svc
	@echo
	@echo "=== Monitoring ==="
	kubectl get pods,svc -n monitoring

port-forward-api:
	kubectl port-forward --address 0.0.0.0 svc/shop-api 8080:80

port-forward-grafana:
	kubectl port-forward --address 0.0.0.0 -n monitoring svc/observability-grafana 3000:80

port-forward-prometheus:
	kubectl port-forward --address 0.0.0.0 -n monitoring svc/observability-kube-prometh-prometheus 9090:9090

port-forward-jaeger:
	kubectl port-forward --address 0.0.0.0 -n monitoring svc/observability-jaeger 16686:16686

port-forward-alertmanager:
	kubectl port-forward --address 0.0.0.0 -n monitoring svc/observability-kube-prometh-alertmanager 9093:9093

clean:
	./cluster/delete.sh