all::
	go build -o esptrans main.go
test:
	go test -v -race ./...

