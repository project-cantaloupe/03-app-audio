.PHONY: test test-api test-worker test-web lint schema validate local-up local-down local-e2e web-install web-dev

test: test-api test-worker test-web

test-api:
	cd services/api && GOTOOLCHAIN=local go test ./...

test-worker:
	cd services/worker && PYTHONPATH=src .venv/bin/pytest -q

test-web:
	cd services/web && npm run build

lint:
	cd services/worker && .venv/bin/ruff check src tests
	cd services/worker && .venv/bin/ruff format --check src tests

schema:
	python3 -m json.tool shared/schema/transcode-job.json >/dev/null
	python3 -m json.tool shared/schema/transcode-result.json >/dev/null
	python3 -m json.tool shared/schema/scan-result.json >/dev/null

validate: test lint schema
	git diff --check

local-up:
	docker compose up --build -d
	docker compose ps

local-down:
	docker compose down

local-e2e:
	./scripts/local-e2e.sh

web-install:
	cd services/web && npm install

web-dev:
	cd services/web && npm run dev
