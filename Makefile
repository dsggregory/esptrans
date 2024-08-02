test:
	go test -v -race ./...

all:
	go build -o esptrans main.go
