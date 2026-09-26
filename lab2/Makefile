CLUSTER_NAME := observability-demo
IMAGE        := api-service
TAG          := latest

.PHONY: help cluster-up cluster-down build image-load deploy port-forward clean

help: ## Показать доступные команды
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

cluster-up: ## Создать Kind-кластер
	./cluster/create.sh

cluster-down: ## Удалить Kind-кластер
	./cluster/delete.sh

build: ## Собрать Docker-образ
	docker build -t $(IMAGE):$(TAG) ./service

image-load: build ## Собрать образ и загрузить в Kind
	kind load docker-image $(IMAGE):$(TAG) --name $(CLUSTER_NAME)

deploy: cluster-up image-load ## Создать кластер, собрать образ и развернуть Helm-чарт
	helm upgrade --install api-service ./charts/api-service \
	  --set image.repository=$(IMAGE) \
	  --set image.tag=$(TAG) \
	  --wait

port-forward: ## Пробросить API на localhost:8080
	kubectl port-forward svc/api-service 8080:80

clean: cluster-down ## Удалить Kind-кластер