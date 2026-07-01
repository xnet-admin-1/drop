.PHONY: all build run clean setup setup-browser test docker-build docker-run

all: build

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o drop .

run: build
	./drop

clean:
	rm -f drop

setup: setup-browser

setup-browser:
	@echo "Setting up browser dependencies..."
	@if [ ! -f scripts/setup-browser.sh ]; then \
		echo "Error: setup-browser.sh not found"; \
		exit 1; \
	fi
	@chmod +x scripts/setup-browser.sh
	@./scripts/setup-browser.sh

test:
	go test -v ./...

docker-build:
	docker build -t drop .

docker-run:
	docker run -it --rm -p 9800:9800 drop

# Development: build for multiple architectures
build-all:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o drop .
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o drop-arm64 .

# Clean everything including browser data
distclean: clean
	rm -rf ~/.config/google-chrome ~/.cache/google-chrome 2>/dev/null || true