.PHONY: build test test-unit test-integration lint clean prometheus grafana dashboards

NAME=openbao-teradata-secret-plugin
VERSION=0.1.0

build:
	go build -o bin/$(NAME) ./cmd/teradata/

test:
	go test -v -count=1 ./...

test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-unit:
	go test -v -count=1 ./...

test-integration:
	@echo "Requires Teradata database running"
	export RUN_INTEGRATION_TESTS=true
	go test -tags=integration -v -count=1 ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

prometheus:
	@echo "Starting Prometheus with teradata-plugin configuration..."
	@echo "Note: Ensure OpenBAO is running and metrics endpoint is accessible"
	@if [ -f /usr/local/bin/prometheus ]; then \
		prometheus --config.file=prometheus.yml --storage.tsdb.path=prometheus_data; \
	else \
		echo "Prometheus not found. Install from https://prometheus.io/download/"; \
	fi

grafana:
	@echo "Grafana dashboard available at: dashboards/grafana/teradata-plugin-overview.json"
	@echo "Import instructions:"
	@echo "  1. Open Grafana"
	@echo "  2. Navigate to Dashboards → Import"
	@echo "  3. Upload dashboards/grafana/teradata-plugin-overview.json"
	@echo "  4. Select Prometheus datasource"
	@echo "  5. Click Import"

dashboards:
	@echo "Dashboard files:"
	@echo "  - Grafana: dashboards/grafana/teradata-plugin-overview.json"
	@echo "  - Prometheus: prometheus.yml"
	@echo "  - Alerts: dashboards/prometheus/alerts/teradata-plugin.yml"
