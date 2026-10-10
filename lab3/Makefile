SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

# Kubernetes / Kind
CLUSTER_NAME ?= observability-demo

# Images
API_IMAGE ?= shop-api
WORKER_IMAGE ?= shop-worker
TAG ?= v1

# Namespaces and Helm releases
APP_NAMESPACE ?= default
MONITORING_NAMESPACE ?= monitoring

CNPG_RELEASE ?= cnpg
CNPG_NAMESPACE ?= cnpg-system

PROMETHEUS_RELEASE ?= monitoring
KARMA_RELEASE ?= karma

# Paths
SHOP_CHART := charts/shop
OBSERVABILITY_DIR := charts/observability
PROMETHEUS_VALUES := $(OBSERVABILITY_DIR)/values/kube-prometheus-stack.yaml
KARMA_VALUES := $(OBSERVABILITY_DIR)/values/karma.yaml

.PHONY: help check-tools check-cluster \
        build-api build-worker build \
        image-load-api image-load-worker image-load \
        install-operator deploy-shop \
        install-prometheus wait-monitoring-crds \
        install-karma verify-monitoring deploy-all

help:
	@echo "Targets:"
	@echo "  make build             Build API and worker images"
	@echo "  make image-load        Load images into Kind"
	@echo "  make install-operator  Install CloudNativePG Operator"
	@echo "  make install-prometheus Install kube-prometheus-stack"
	@echo "  make deploy-shop       Deploy the shop Helm chart"
	@echo "  make install-karma     Install Karma"
	@echo "  make deploy-all        Install the complete lab stack"

check-tools:
	@command -v docker >/dev/null || { echo "ERROR: docker not found"; exit 1; }
	@command -v kind >/dev/null || { echo "ERROR: kind not found"; exit 1; }
	@command -v kubectl >/dev/null || { echo "ERROR: kubectl not found"; exit 1; }
	@command -v helm >/dev/null || { echo "ERROR: helm not found"; exit 1; }

check-cluster: check-tools
	@kind get clusters | grep -Fxq "$(CLUSTER_NAME)" || { \
		echo "ERROR: Kind cluster '$(CLUSTER_NAME)' does not exist."; \
		echo "Create it first using the project's cluster configuration."; \
		exit 1; \
	}
	@kubectl cluster-info >/dev/null
	@echo "Kind cluster $(CLUSTER_NAME) is reachable."

build-api:
	docker build -t $(API_IMAGE):$(TAG) ./service/api

build-worker:
	docker build -t $(WORKER_IMAGE):$(TAG) ./service/worker

build: build-api build-worker

image-load-api:
	kind load docker-image $(API_IMAGE):$(TAG) --name $(CLUSTER_NAME)

image-load-worker:
	kind load docker-image $(WORKER_IMAGE):$(TAG) --name $(CLUSTER_NAME)

image-load: image-load-api image-load-worker

install-operator: check-cluster
	helm repo add cloudnative-pg https://cloudnative-pg.github.io/charts
	helm repo update
	kubectl create namespace $(CNPG_NAMESPACE) \
		--dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install $(CNPG_RELEASE) cloudnative-pg/cloudnative-pg \
		--namespace $(CNPG_NAMESPACE) \
		--wait --timeout 5m
	@echo "Waiting for CloudNativePG CRD..."
	@for i in $$(seq 1 60); do \
		kubectl get crd clusters.postgresql.cnpg.io >/dev/null 2>&1 && break; \
		sleep 2; \
	done
	@kubectl get crd clusters.postgresql.cnpg.io >/dev/null
	@echo "CloudNativePG CRD is available."

install-prometheus: check-cluster
	@test -f "$(PROMETHEUS_VALUES)" || { \
		echo "ERROR: missing $(PROMETHEUS_VALUES)"; exit 1; \
	}
	helm repo add prometheus-community \
		https://prometheus-community.github.io/helm-charts
	helm repo update
	kubectl create namespace $(MONITORING_NAMESPACE) \
		--dry-run=client -o yaml | kubectl apply -f -
	helm upgrade --install $(PROMETHEUS_RELEASE) \
		prometheus-community/kube-prometheus-stack \
		--namespace $(MONITORING_NAMESPACE) \
		-f $(PROMETHEUS_VALUES) \
		--wait --timeout 10m
	$(MAKE) wait-monitoring-crds

wait-monitoring-crds:
	@echo "Waiting for ServiceMonitor and PrometheusRule CRDs..."
	@for crd in servicemonitors.monitoring.coreos.com prometheusrules.monitoring.coreos.com; do \
		for i in $$(seq 1 60); do \
			kubectl get crd "$$crd" >/dev/null 2>&1 && break; \
			sleep 2; \
		done; \
		kubectl get crd "$$crd" >/dev/null || { \
			echo "ERROR: CRD $$crd was not installed"; exit 1; \
		}; \
	done
	@echo "Monitoring CRDs are available."

deploy-shop: check-cluster wait-monitoring-crds
	helm upgrade --install shop $(SHOP_CHART) \
		--namespace $(APP_NAMESPACE) \
		--create-namespace \
		--set api.image.repository=$(API_IMAGE) \
		--set api.image.tag=$(TAG) \
		--set worker.image.repository=$(WORKER_IMAGE) \
		--set worker.image.tag=$(TAG) \
		--wait --timeout 10m

install-karma: check-cluster
	@test -f "$(KARMA_VALUES)" || { \
		echo "ERROR: missing $(KARMA_VALUES)"; \
		echo "Create the Karma values file before installing Karma."; \
		exit 1; \
	}
	helm repo add wiremind https://wiremind.github.io/wiremind-helm-charts
	helm repo update
	helm upgrade --install $(KARMA_RELEASE) wiremind/karma \
		--namespace $(MONITORING_NAMESPACE) \
		-f $(KARMA_VALUES) \
		--wait --timeout 5m

verify-monitoring:
	@echo "=== Kubernetes resources ==="
	kubectl get pods -n $(MONITORING_NAMESPACE)
	kubectl get servicemonitor -A
	kubectl get prometheusrule -A
	@echo "=== Shop resources ==="
	kubectl get pods -n $(APP_NAMESPACE)
	kubectl get svc shop-api -n $(APP_NAMESPACE)
	kubectl get endpoints shop-api -n $(APP_NAMESPACE)

deploy-all: check-cluster install-operator build image-load install-prometheus deploy-shop install-karma
	$(MAKE) verify-monitoring
	@echo "Deployment completed."
	@echo "Prometheus: kubectl port-forward -n $(MONITORING_NAMESPACE) svc/$(PROMETHEUS_RELEASE)-kube-prometheus-prometheus 9090:9090"