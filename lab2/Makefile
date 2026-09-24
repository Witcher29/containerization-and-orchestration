CLUSTER_NAME := observability-demo
IMAGE        := api-service
TAG          := latest

.PHONY: help cluster-up cluster-down image-load deploy port-forward clean

help:                ## Показать доступные команды
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

cluster-up:          ## Создать кластер
	./cluster/create.sh

cluster-down:        ## Удалить кластер
	./cluster/delete.sh

build:               ## Собрать Docker-образ
	docker build -t $(IMAGE):$(TAG) ./service

image-load: build    ## Собрать образ и загрузить в kind
	kind load docker-image $(IMAGE):$(TAG) --name $(CLUSTER_NAME)

deploy: image-load   ## Развернуть Helm-чарт
	helm upgrade --install api-service ./charts/api-service \
	  --set image.repository=$(IMAGE) \
	  --set image.tag=$(TAG) \
	  --wait

port-forward:        ## Пробросить порт сервиса на localhost:8080
	kubectl port-forward svc/api-service-api-service 8080:80

clean: cluster-down  ## Удалить всё
	rm -f api-service