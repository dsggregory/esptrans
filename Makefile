all:: views/styles/style.css
	go build -o esptrans cmd/esptrans/main.go
	go build -o server cmd/server/main.go
test:
	go test -v -race ./...
tailwindcss:
	curl -sLO https://github.com/tailwindlabs/tailwindcss/releases/latest/download/tailwindcss-macos-arm64
	chmod +x tailwindcss-macos-arm64
	mv tailwindcss-macos-arm64 tailwindcss
	# creates ./tailwind.config.js
	#./tailwindcss init

views/styles/style.css: ./views/styles/style.css.in tailwindcss
	# rebuilds the main style.css to include tailwind artifacts
	./tailwindcss -i ./views/styles/style.css.in -o $@ --minify
