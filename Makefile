.PHONY: build install clean test generate-plugins analyze-repo

# Версия и инфо
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION ?= $(shell go version | cut -d " " -f 3)

# Флаги линковки
LDFLAGS = -ldflags "\
	-X main.Version=$(VERSION) \
	-X main.BuildTime=$(BUILD_TIME) \
	-X main.GoVersion=$(GO_VERSION) \
	-s -w"

# Основные цели
build: alr-updater analyze-repo

alr-updater:
	@echo "🔨 Building ALR-updater..."
	CGO_ENABLED=1 go build $(LDFLAGS) -o alr-updater main.go

analyze-repo:
	@echo "🔨 Building repository analyzer..."
	cd cmd/analyze-repo && CGO_ENABLED=1 go build $(LDFLAGS) -o ../../analyze-repo .

install: build
	@echo "📦 Installing ALR-updater..."
	sudo install -m 755 alr-updater /usr/local/bin/
	sudo install -m 755 analyze-repo /usr/local/bin/
	@echo "✅ Installation complete!"

clean:
	@echo "🧹 Cleaning..."
	rm -f alr-updater analyze-repo

test:
	@echo "🧪 Running tests..."
	go test -v ./...

# Генерация плагинов
generate-plugins:
	@echo "🤖 Generating missing plugins..."
	./alr-updater --generate-plugins

# Анализ репозитория
analyze:
	@echo "📊 Analyzing repository..."
	./analyze-repo --format=table

analyze-json:
	@echo "📊 Analyzing repository (JSON)..."
	./analyze-repo --format=json

# Генерация через анализатор
generate-missing:
	@echo "🤖 Generating missing plugins via analyzer..."
	./analyze-repo --generate

# Помощь
help:
	@echo "ALR-updater Build System"
	@echo "========================"
	@echo ""
	@echo "Targets:"
	@echo "  build           - Build all binaries"
	@echo "  alr-updater     - Build main updater"
	@echo "  analyze-repo    - Build repository analyzer"
	@echo "  install         - Install binaries to /usr/local/bin"
	@echo "  clean           - Remove built binaries"
	@echo "  test            - Run tests"
	@echo ""
	@echo "Plugin Generation:"
	@echo "  generate-plugins - Generate missing plugins"
	@echo "  analyze         - Analyze repository (table format)"
	@echo "  analyze-json    - Analyze repository (JSON format)"
	@echo "  generate-missing - Analyze and generate missing plugins"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION         - Version string (default: git describe)"
	@echo "  BUILD_TIME      - Build timestamp"

# Цель по умолчанию
all: build