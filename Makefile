all::
	go build -o esptrans cmd/esptrans/main.go
test:
	go test -v -race ./...
tailwindcss:
	curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64
	chmod +x tailwindcss-macos-arm64
	mv tailwindcss-macos-arm64 tailwindcss
	# creates ./tailwind.config.js
	#./tailwindcss init
tailwind: tailwindcss
	# rebuilds the main style.css to include tailwind artifacts
	./tailwindcss -i ./views/styles/style.css.in -o ./views/styles/style.css --minify
