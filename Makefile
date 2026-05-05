.PHONY: dev build build-web build-server run

dev:
	@trap 'kill 0' EXIT; \
	go run ./cmd/server -data data -web web/dist & \
	cd web && npm run dev

build: build-web build-server

build-web:
	cd web && npm run build

build-server:
	go build -o bin/server ./cmd/server

run: build
	./bin/server -data data -web web/dist
