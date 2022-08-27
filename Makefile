compile-protoc:
	protoc api/v1/*.proto --go_out=. --proto_path=.
test:
	go test --race ./...