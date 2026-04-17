# Подключаем переменные из файла .env
ifneq (,$(wildcard ./.env))
    include .env
    export
endif

.PHONY: api-gen up down seed setup copy-env copy-config

setup: copy-env copy-config

copy-env:
	cp .example.env .env

copy-config:
	cp config.example.yaml config.yaml

# Генерация кода из OpenAPI
api-gen:
	oapi-codegen -config=oapi-codegen.yaml api.yaml 

# Запуск сервисов в Docker
up:
	docker-compose up --build

# Остановка сервисов (исправлено dwon -> down)
down:
	docker-compose down

# Наполнение базы данными
# Берем DATABASE_URL из .env. Если её там нет, можно передать вручную: make seed DB_URL=...
seed:
	@if [ -z "$(DATABASE_URL)" ]; then \
		echo "Error: DATABASE_URL is not set in .env file"; \
		exit 1; \
	fi
	go -C ./app run seed/main.go --db-url="$(DATABASE_URL)"

make load-test:
	docker run --rm -i --network=host grafana/k6 run - <load-test.js