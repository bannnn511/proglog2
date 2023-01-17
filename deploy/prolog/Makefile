create-cluster:
	kind create cluster --config ./config/cluster-local.yaml
install:
	kubectl apply -f ./config/storage-class.yaml && \
	kubectl apply -f ./config/persistent-volume.yaml && \
	kubectl apply -f ./config/persistent-volume-1.yaml && \
	kubectl apply -f ./config/persistent-volume-2.yaml && \
	helm install prolog .
clean:
	kubectl delete -f ./config/storage-class.yaml && \
	kubectl delete -f ./config/persistent-volume.yaml && \
	kubectl delete -f ./config/persistent-volume-1.yaml && \
	kubectl delete -f ./config/persistent-volume-2.yaml && \
	helm delete prolog && \
	kubectl delete pvc datadir-prolog-0 \
	kubectl delete pvc datadir-prolog-1 \
	kubectl delete pvc datadir-prolog-2