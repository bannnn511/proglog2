TAG ?= 0.0.4
REGION ?= asia-southeast1

set-project-id:
	export PROJECT_ID=$(cloud projects list | grep 'distributed' |tail -n 1 | cut -d' ' -f1)
# START: Docker
build-binary:
	go build -o ./prolog ./cmd/prolog/
build-image:
	docker build . -t bannnnn/prolog:$(TAG)
build-docker:
	docker build . -t bannnnn/prolog:$(TAG) && docker push bannnnn/prolog:$(TAG)
build-docker-gcp:
    # https://cloud.google.com/artifact-registry/docs/docker/pushing-and-pulling
    # docker tag bannnnn/prolog:0.0.4 asia-southeast1-docker.pkg.dev/distributed-log-376410/prolog/prolog:0.0.4
    # docker push asia-southeast1-docker.pkg.dev/distributed-log-376410/prolog/prolog:0.0.4
	docker tag bannnnn/prolog:$(TAG) $(REGION)-docker.pkg.dev/${PROJECT_ID}/prolog/prolog:$(TAG) && \
	docker push $(REGION)-docker.pkg.dev/${PROJECT_ID}/prolog/prolog:$(TAG)
run-prolog:
	docker build . -t prolog:$(TAG) && \
	docker run prolog:$(TAG)
clean-docker:
	docker ps -a | grep 'prolog' | awk '{print $1}' | xargs docker rm
	docker images -a | grep "prolog" | awk '{print $3}' | xargs docker rmi
# END: Docker

# START: protobuf
compile-protoc:
	protoc api/v1/*.proto \
                    --go_out=. \
                    --go-grpc_out=. \
                    --go_opt=paths=source_relative \
                    --go-grpc_opt=paths=source_relative \
                    --proto_path=.
# END: protobuf

test:
	go test --race ./...

# START: cert
.PHONY: gencert
gencert:
	# START: init ca
	cfssl gencert -initca ./config/certs/ca-csr.json | cfssljson -bare ca

	cfssl gencert -ca=ca.pem \
					-ca-key=ca-key.pem\
					 -config=./config/certs/ca-config.json\
					  -profile=server \
					  ./config/certs/server-csr.json | cfssljson -bare server
	# END: init ca

	# START: client
	cfssl gencert -ca=ca.pem \
					-ca-key=ca-key.pem\
					 -config=./config/certs/ca-config.json\
					  -profile=client \
					  ./config/certs/client-csr.json | cfssljson -bare root-client

	# END: client

	# START: multi client
	cfssl gencert -ca=ca.pem \
					-ca-key=ca-key.pem\
					 -config=./config/certs/ca-config.json\
					  -profile=client \
					  ./config/certs/client-csr.json | cfssljson -bare nobody-client
	# END: multi client

	# START: begin
	mv *.csr *.pem ./config/certs/
	# END: begin

clean-cert:
	rm -rf ./config/certs/*.csr ./config/certs/*.pem
# END: cert
