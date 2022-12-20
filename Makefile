
compile-protoc:
	protoc api/v1/*.proto \
                    --go_out=. \
                    --go-grpc_out=. \
                    --go_opt=paths=source_relative \
                    --go-grpc_opt=paths=source_relative \
                    --proto_path=.
test:
	go test --race ./...

gencert:
	# START: initca
	cfssl genkey -initca ./config/certs/ca-csr.json | cfssljson -bare ca

	cfssl gencert -ca=ca.pem \
					-ca-key=ca-key.pem\
					 -config=./config/certs/ca-config.json\
					  -profile=server \
					  ./config/certs/server-csr.json | cfssljson -bare server

	mv *.csr *.pem ./config/certs/
	# END: initca

	# START: client
	cfssl genkey -initca ./config/certs/ca-csr.json | cfssljson -bare ca

	cfssl gencert -ca=ca.pem \
					-ca-key=ca-key.pem\
					 -config=./config/certs/ca-config.json\
					  -profile=client \
					  ./config/certs/client-csr.json | cfssljson -bare root-client

	mv *.csr *.pem ./config/certs/
	# END: client

	# START: multi client
	cfssl genkey -initca ./config/certs/ca-csr.json | cfssljson -bare ca

	cfssl gencert -ca=ca.pem \
					-ca-key=ca-key.pem\
					 -config=./config/certs/ca-config.json\
					  -profile=client \
					  ./config/certs/client-csr.json | cfssljson -bare nobody-client
	# END: multi client

	# START: begin
	mv *.csr *.pem ./config/certs/
	# END: begin
