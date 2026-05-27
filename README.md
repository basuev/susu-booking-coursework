# booking-svc

Микросервис бронирования гостиниц. Go 1.26, gRPC, PostgreSQL, NATS JetStream,
«transactional outbox» и «CQRS»-проекция. Послойная структура по принципам
чистой архитектуры и предметно-ориентированного проектирования (DDD).

## Быстрый старт

Требуются Docker, Go 1.26 и `buf` (только для перегенерации proto).

```bash
docker compose up -d                  # postgres + nats
go run ./cmd/booking-svc              # auto-applies migrations on startup
```

Проверка работоспособности:

```bash
curl -fsS localhost:8080/healthz
```

Порты по умолчанию: gRPC «:50051», HTTP health «:8080».

## Архитектура

```
                    +-------------------+
   gRPC client ---> | port/grpc         |
                    | BookingHandler    |
                    +---------+---------+
                              |
                    +---------v---------+
                    | app/command       |
                    | app/query         |
                    +---------+---------+
                              |
                    +---------v---------+
                    | domain/booking    |
                    | aggregate, FSM,   |
                    | value objects     |
                    +---------+---------+
                              |
       +----------------------+----------------------+
       |                                             |
+------v-------+                              +------v-------+
| postgres     |  --writes booking + outbox-->| outbox       |
| BookingRepo  |    same tx                   | worker       |
| ProjectionRepo|                             +------+-------+
+--------------+                                     |
       ^                                             v
       |                                       +-----+------+
       |          projector worker  <----------|  NATS      |
       |           consumes events             |  JetStream |
       +---------------------------------------+------------+
```

Путь записи: gRPC-запрос — обработчик команды — доменный агрегат —
`BookingRepo.Save` в транзакции, которая попутно записывает строку
`outbox_message`. Фоновый outbox-worker вычитывает «pending»-строки через
`UPDATE ... RETURNING` и публикует их в NATS JetStream.

Путь чтения: `List` читает из `booking_projection` (с конечной
согласованностью), которая наполняется отдельным projector-worker'ом,
подписанным на `booking-events`. `GetBooking` читает строго согласованное
состояние напрямую из модели записи.

Подробнее: см. [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). API: см.
[docs/api.md](docs/api.md). Архитектурные решения: [docs/adr/](docs/adr/).

## Паттерны

- «Transactional outbox»: состояние агрегата и `outbox_message` пишутся в
  одной SQL-транзакции (`internal/adapter/postgres/booking_repo.go`).
  Worker забирает «pending»-строки через `UPDATE ... RETURNING` (атомарный
  захват), публикует их в NATS JetStream и помечает как опубликованные. При
  сбоях увеличивается `attempt_count`; при превышении `OUTBOX_MAX_ATTEMPTS`
  строка переводится в `DLQ`.
- «CQRS»-модель чтения: таблица `booking_projection` обслуживает
  `ListBookings`. Ее строит `internal/adapter/projector/worker.go`,
  подписанный на `booking-events` с durable-consumer'ом.
- «Optimistic locking» (оптимистическая блокировка): `booking.version`
  проверяется в `WHERE` каждого `UPDATE`. Конфликт превращается в
  `domain.ErrConcurrentUpdate` и далее в gRPC `Aborted`.
- Идемпотентное создание: `idempotency_key` хранится вместе с хешем запроса
  и итоговым `booking_id`. Повтор с тем же ключом и тем же payload'ом
  возвращает тот же id; другой payload с тем же ключом возвращает
  `AlreadyExists`.

## Сборка, тесты, линт

```bash
make build            # compile cmd/booking-svc
make test-unit        # go test -race ./... (no integration tag)
make test-integration # go test -race -tags=integration ./... (needs docker)
make coverage         # writes coverage.html
make lint             # golangci-lint run ./...
make fmt              # gofumpt + goimports if installed
make proto            # buf generate
```

Более низкоуровневые команды описаны в `Makefile`.

## Health check'и

| Endpoint | Описание |
|----------|----------|
| `GET /healthz` | liveness, всегда возвращает 200 |
| `GET /readyz` | readiness: пингует db и nats, 200 если оба ok, иначе 503 |
| gRPC `grpc.health.v1.Health/Check` | стандартный gRPC health |

## Конфигурация

Все настройки берутся из env vars (12-factor). Значения по умолчанию —
в `internal/config/config.go`.

| Переменная | По умолчанию | Примечания |
|------------|--------------|------------|
| `GRPC_PORT` | `50051` | порт gRPC |
| `HTTP_ADDR` | `:8080` | адрес health/HTTP |
| `DATABASE_URL` | `postgres://booking:booking@localhost:5432/booking?sslmode=disable` | DSN, совместимый с pgx |
| `NATS_URL` | `nats://localhost:4222` | URL сервера NATS |
| `LOG_LEVEL` | `info` | `debug` / `info` / `warn` / `error` |
| `DB_MAX_OPEN_CONNS` | `25` | sql.DB.SetMaxOpenConns |
| `DB_MAX_IDLE_CONNS` | `5` | sql.DB.SetMaxIdleConns |
| `DB_CONN_MAX_LIFETIME` | `30m` | sql.DB.SetConnMaxLifetime |
| `OUTBOX_BATCH_SIZE` | `50` | строк, забираемых за один тик |
| `OUTBOX_INTERVAL` | `500ms` | интервал опроса |
| `OUTBOX_MAX_ATTEMPTS` | `10` | число попыток до `DLQ` |
| `SHUTDOWN_TIMEOUT` | `10s` | дедлайн мягкого завершения |
| `RPC_TIMEOUT` | `30s` | серверный дедлайн на один RPC |
| `RUN_MIGRATIONS` | `true` | автоматически применять миграции при старте |

## Структура

```
cmd/booking-svc/                   main + composition root
internal/
  domain/booking/                  aggregate, FSM, value objects, events
  app/command/                     write-side use cases
  app/query/                       read-side use cases
  adapter/postgres/                BookingRepo, ProjectionRepo, IdempotencyRepo, TxManager
  adapter/outbox/                  outbox repo + worker
  adapter/projector/               projection worker
  adapter/natsx/                   JetStream client wrapper
  port/grpc/                       gRPC server, handlers, interceptors, mapper
  port/http/                       health endpoints
  config/                          env-driven config
pkg/id/                            uuid helpers
api/proto/booking/v1/              proto sources
gen/go/booking/v1/                 generated Go (checked in)
migrations/                        embedded sql, applied on startup
deployments/docker/                Dockerfile
.gitverse/workflows/               CI definitions
docs/                              architecture, ADRs, API
```
