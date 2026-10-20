.PHONY: test run migrate tidy build-web

test:
	go test ./...

tidy:
	go mod tidy

run:
	go run ./cmd/app -config configs/config.yaml

migrate:
	go run github.com/pressly/goose/v3/cmd/goose@latest -dir migrations postgres "$(DATABASE_DSN)" up

build-web:
	cd web && npm install && npm run build
	@echo "Copy web/dist into cmd/app/static (or rely on Docker multi-stage build)"
