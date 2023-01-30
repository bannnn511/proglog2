TAG ?= 0.0.4

build:
	go build -o ./prolog ./cmd/prolog/
build-docker:
	docker build . -t bannnnn/prolog:$(TAG) && docker push bannnnn/prolog:$(TAG)
run-prolog:
	docker build . -t prolog:$(TAG) && \
	docker run prolog:$(TAG)
clean-docker:
	docker ps -a | grep 'prolog' | awk '{print $1}' | xargs docker rm
	docker images -a | grep "prolog" | awk '{print $3}' | xargs docker rmi

compile-protoc:
	protoc api/v1/*.proto \
                    --go_out=. \
                    --go-grpc_out=. \
                    --go_opt=paths=source_relative \
                    --go-grpc_opt=paths=source_relative \
                    --proto_path=.
test:
	go test --race ./...

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