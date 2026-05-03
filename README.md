# Shopping Backend (Go)

Фундамент для бэкенда менеджера списка покупок (Flutter клиент).

## Что уже есть

- HTTP API каркас (пока только `GET /health`)
- Конфиг из переменных окружения (`.env`)
- Подключение к Postgres (pool)
- Миграции через `golang-migrate`
- Graceful shutdown
- `Dockerfile` (multi-stage, scratch runtime)
- `docker-compose.yml` для локального запуска Postgres + app
- GitHub Actions для сборки и пуша образа в GHCR

## Локальный запуск

1) Скопируй env:

```bash
cp .env.example .env
```

2) Подними сервисы:

```bash
docker-compose up --build
```

3) Проверка:

```bash
curl -s http://localhost:8080/health
```

## Миграции

- Миграции лежат в `./migrations`
- По умолчанию app может прогонять миграции при старте (см. `RUN_MIGRATIONS=true`)

Для ручного запуска миграций из контейнера/локально можно использовать CLI `migrate`:

```bash
migrate -path migrations -database "$DB_URL" up
```

