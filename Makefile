all::
	go build -o esptrans cmd/esptrans/main.go
test:
	go test -v -race ./...

