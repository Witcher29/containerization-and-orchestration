.PHONY: help cluster-up calico deploy test-network load fault restore status port-forward-api port-forward-prometheus port-forward-grafana cluster-down

help:
	@printf '%s\n' 'cluster-up   Create isolated kind cluster' 'calico       Install Calico CNI' 'deploy       Install operator, monitoring, app and policies' 'test-network Verify allow/deny matrix' 'load         Run k6 load test' 'fault        Inject temporary API egress fault' 'restore      Restore Helm-managed network policies' 'status       Show nodes, pods, services and policies' 'cluster-down Delete only lab5-calico'

cluster-up:
	./cluster/create.sh
calico:
	./cluster/install-calico.sh
deploy:
	./scripts/deploy.sh
test-network:
	./scripts/test-connectivity.sh
load:
	./scripts/load-test.sh
fault:
	./scripts/inject-network-fault.sh
restore:
	./scripts/restore-network.sh
status:
	kubectl config use-context kind-lab5-calico
	kubectl get nodes -o wide
	kubectl get pods,svc,networkpolicy -A
cluster-down:
	./cluster/delete.sh
port-forward-api:
	kubectl port-forward --address 127.0.0.1 svc/shop-api 8080:80
port-forward-prometheus:
	kubectl port-forward --address 127.0.0.1 -n monitoring svc/observability-kube-prometh-prometheus 9090:9090
port-forward-grafana:
	kubectl port-forward --address 127.0.0.1 -n monitoring svc/observability-grafana 3000:80
