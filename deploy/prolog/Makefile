build-init:
	docker build -t bannnnn/init-prolog ./config && docker push bannnnn/init-prolog
install:
	kubectl apply -f ./config/storage-class.yaml && helm install prolog .