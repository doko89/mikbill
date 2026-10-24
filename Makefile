.PHONY: frontend build build-arm build-be dev-be dev-fe test test-go clean

frontend:
	cd frontend && npm ci && npm run build && rm -rf ../server/web/dist && cp -r dist ../server/web/dist

build: frontend
	cd server && go build  -ldflags "-s -w" -trimpath -o ../bin/mikbil ./cmd/mikbil

build-arm: frontend
	cd server && GOOS=linux GOARCH=arm64 go build -ldflags "-s -w" -trimpath -o ../bin/mikbil-arm64 ./cmd/mikbil

build-be:
	cd server && go build  -ldflags "-s -w" -trimpath -o ../bin/mikbil ./cmd/mikbil

test: test-go

test-go:
	cd server && go test ./...

dev-be:
	cd server && go run ./cmd/mikbil serve

dev-fe:
	cd frontend && npm run dev

clean:
	rm -rf bin frontend/dist frontend/node_modules/.vite
