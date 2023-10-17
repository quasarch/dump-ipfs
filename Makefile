TELEPRESENCE := datawire/tel2:2.13.1
POSTGRES     := ghusta/postgres-world-db:2.10
KIND_CLUSTER := starter-cluster

VERSION := v0.0.1

db-apply:
	kubectl apply -f deployment.yaml

db-update:
	docker build \
    		-f pg-backup-ipfs/Dockerfile \
    		-t pg-backup-ipfs:$(VERSION) \
    		./pg-backup-ipfs
	kind load docker-image pg-backup-ipfs:$(VERSION) --name $(KIND_CLUSTER)
	kubectl apply -f deployment.yaml
	kubectl rollout restart statefulset database --namespace=eth-system

db-test-dump:
	go run pg-backup-ipfs/main.go postgresql://world:world123@database.eth-system.svc.cluster.local:5432/world-db
# Cluster
dev-up-local:
	kind create cluster \
		--image kindest/node:v1.26.3@sha256:61b92f38dff6ccc29969e7aa154d34e38b89443af1a2c14e6cfbd2df6419c66f \
		--name $(KIND_CLUSTER) \
		--config kind-config.yaml

	kubectl wait --timeout=120s --namespace=local-path-storage --for=condition=Available deployment/local-path-provisioner

	kind load docker-image $(TELEPRESENCE) --name $(KIND_CLUSTER)
	kind load docker-image $(POSTGRES) --name $(KIND_CLUSTER)

dev-up: dev-up-local
	telepresence --context=kind-$(KIND_CLUSTER) helm install
	telepresence --context=kind-$(KIND_CLUSTER) connect

dev-down-local:
	kind delete cluster --name $(KIND_CLUSTER)

dev-down:
	telepresence quit -s
	kind delete cluster --name $(KIND_CLUSTER)
