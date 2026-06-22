# File Storage Server

Сервер для хранения файлов с личными и глобальными хранилищами, папками, правами доступа, публичными ссылками на скачивание/редактирование, web-интерфейсом и JSON API.

## Структура

```text
cmd/                 точки входа: app, migrate, admin
internal/domain/     доменные модели
internal/*/          доменные пакеты: auth, storage, file, folder, access, share
internal/api/        JSON API handlers and routing
internal/web/        HTML handlers, web helpers and page rendering
internal/filesystem/ локальное файловое хранилище
internal/postgres/   PostgreSQL connection and transactions
internal/middleware/ HTTP middleware
migrations/          SQL миграции goose
templates/           HTML templates
static/              CSS/JS assets
```

## Локальный запуск

```bash
cp .env.example .env
docker compose up -d postgres
go run ./cmd/migrate up
go run ./cmd/admin -email admin@example.com -username admin -password password123
go run ./cmd/app
```

Приложение будет доступно на `http://localhost:8080`.

## Проверки

```bash
go test ./...
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
```

## Docker Compose

```bash
docker compose up --build
```

Compose поднимает PostgreSQL, применяет миграции и запускает приложение.

## Cleanup

Фоновая job очищает истекшие refresh tokens, истекшие публичные ссылки, старые временные файлы и orphan-файлы в `files/`, которых уже нет в активных строках БД. Интервалы настраиваются через `CLEANUP_INTERVAL`, `STAGED_FILE_TTL` и `ORPHAN_FILE_TTL`.
