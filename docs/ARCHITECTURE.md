# Архитектура

## Слои

Сервис построен по принципам чистой архитектуры со строительными блоками
DDD. Правило зависимостей: внешние слои зависят от внутренних, внутренние
ничего не знают о внешних. Адаптеры зависят от портов (Go-интерфейсов,
объявленных внутренним слоем); composition root в
`cmd/booking-svc/main.go` связывает конкретные адаптеры с приложением.

```
+-----------------------------------------------------------+
|  port/grpc                  port/http                     |  transport
|  BookingHandler             healthz                       |
+-----------------------------+-----------------------------+
|  app/command                app/query                     |  application
|  CreateBooking, ...         GetBooking, ListBookings      |
+-----------------------------+-----------------------------+
|                       domain/booking                      |  domain
|        Booking aggregate, Status FSM, Money,              |
|        OfferSnapshot, StayPeriod, Events                  |
+-----------------------------------------------------------+
^                                                           ^
|  ports (Go interfaces declared inside app/ and domain/)   |
v                                                           v
+-----------------------------+-----------------------------+
|  adapter/postgres           adapter/outbox                |  adapters
|  BookingRepo                Repo, Worker                  |
|  ProjectionRepo             adapter/projector             |
|  IdempotencyRepo            Worker                        |
|  TxManager                  adapter/natsx                 |
+-----------------------------------------------------------+
```

Направление импортов:

- `domain/*` ничего не импортирует из `app`, `adapter` или `port`.
- `app/*` импортирует `domain/*` и объявляет собственные порты
  (например, `app/command/ports.go` определяет `BookingRepo`,
  `IdempotencyRepo`).
- `adapter/*` импортирует `domain/*` и удовлетворяет порты `app`.
- `port/*` импортирует `app/*` и транслирует транспорт в use case'ы.
- `cmd/booking-svc/main.go` — единственное место, где конкретные адаптеры
  и порты импортируются явно.

## Агрегат Booking

`internal/domain/booking/booking.go`. Агрегат хранит:

- `id`, `guestID`
- `offer` (`OfferSnapshot`: offerID, hotelID, roomType, price)
- `stay` (`StayPeriod`: checkIn, checkOut, обрезанные до даты)
- `total` (`Money`: int64 в минимальных единицах + валюта ISO 4217)
- `status` (`Status`: состояние FSM)
- `version` (int, токен оптимистической блокировки)
- `createdAt`, `updatedAt`
- буфер `events`, который инфраструктура вычитывает через `Events()` /
  `ClearEvents()` после успешного сохранения

Конструктор (`NewBooking`) записывает событие `BookingCreated`.
`Reconstruct` восстанавливает агрегат из персистентного состояния и
событий не эмитит.

### FSM статусов

```
              +------------+
              |  PENDING   |  initial
              +-----+------+
                    |
   +----------------+----------------+----------------+
   |                |                |                |
   v                v                v                v
+------+      +---------+      +---------+      +---------+
|CONF. |      |APPROVED |      |REJECTED |      |CANCELLED|
+--+---+      +---------+      +---------+      +---------+
   |          terminal         terminal         terminal
   +--------+----------+----------+
            |          |          |
            v          v          v
        APPROVED   REJECTED   CANCELLED
```

Разрешенные переходы (`internal/domain/booking/status.go`):

| Из | Допустимые `To` |
|------|--------------|
| `PENDING` | `CONFIRMED`, `APPROVED`, `REJECTED`, `CANCELLED` |
| `CONFIRMED` | `APPROVED`, `REJECTED`, `CANCELLED` |
| `APPROVED` | (terminal) |
| `REJECTED` | (terminal) |
| `CANCELLED` | (terminal) |

Любой другой переход возвращает `domain.ErrInvalidTransition`, который
маппится в gRPC `FailedPrecondition`.

### События

При записи агрегат сохраняет события в памяти:

- `BookingCreated`
- `BookingCancelled`
- `BookingApproved`
- `BookingRejected`

Postgres-адаптер вычитывает список событий внутри той же транзакции, в
которой сохраняется агрегат, и складывает каждое событие как строку
`outbox_message`. `ClearEvents()` вызывается после успешного коммита.

## Сквозной поток записи

```
client                 grpc handler              command handler
  |                          |                          |
  |---CreateBookingRequest-->|                          |
  |                          |--CreateBooking{...}----->|
  |                          |                          |
  |                          |                  +-------v---------+
  |                          |                  | tx begin        |
  |                          |                  |                 |
  |                          |                  | idempo lookup   |
  |                          |                  |   hit -> return |
  |                          |                  |   miss -> next  |
  |                          |                  |                 |
  |                          |                  | NewBooking      |
  |                          |                  | repo.Save       |
  |                          |                  |  upsert booking |
  |                          |                  |  upsert offer   |
  |                          |                  |  append history |
  |                          |                  |  append outbox  |
  |                          |                  |                 |
  |                          |                  | idempo store    |
  |                          |                  | tx commit       |
  |                          |                  +-------+---------+
  |                          |<----Booking aggregate----|
  |<--CreateBookingResponse--|                          |

outbox worker                 nats jetstream             projector
  |                                  |                         |
  |--UPDATE...RETURNING claim-->     |                         |
  |  (atomic, only-once)             |                         |
  |--publish booking-events.*------->|                         |
  |--mark published-->               |                         |
  |                                  |--durable consumer------>|
  |                                  |                         |--upsert projection
  |                                  |                         |--ack
```

Соответствующий код:

- `internal/app/command/create_booking.go` оркеструет транзакцию через
  `TxManager`.
- `internal/adapter/postgres/booking_repo.go` выполняет четыре upsert'а
  внутри одного `*sql.Tx`. `appendOutbox` сериализует каждое событие в
  json и вставляет строку со `status='PENDING'`.
- `internal/adapter/outbox/repo.go` захватывает «pending»-строки одним
  оператором
  `UPDATE ... WHERE id IN (SELECT ... FOR UPDATE SKIP LOCKED) RETURNING ...`,
  устраняя гонку read/write, на которую мог бы наткнуться сторонний
  worker.
- `internal/adapter/outbox/publisher.go` публикует с повторами, увеличивает
  `attempt_count` и переводит строки в `DLQ` после `OUTBOX_MAX_ATTEMPTS`.
- `internal/adapter/projector/worker.go` подписывается на
  `booking-events.*` через durable-consumer и вызывает
  `ProjectionRepo.Upsert`.

## Компромиссы по согласованности

- `GetBooking` читает через `BookingRepo.FindByID` (модель записи). Чтение
  строго согласованное: любое изменение состояния, наблюдавшееся в
  предыдущем успешном RPC, видно в последующем `GetBooking`.
- `ListBookings` читает из `booking_projection`. Проекция согласована в
  конечном счете: только что созданное бронирование может не появиться,
  пока projector не обработает соответствующее событие из outbox. Лаг
  ограничен `OUTBOX_INTERVAL` плюс round-trip до NATS плюс время
  обработки в projector'е (обычно меньше секунды).
- Параллельные смены состояния защищены оптимистической блокировкой:
  каждый `UPDATE` содержит `WHERE version = $expected`. Если строка
  уже обновлена, `RowsAffected == 0`, и репозиторий возвращает
  `domain.ErrConcurrentUpdate`. Обработчик не повторяет операцию; клиенты
  получают gRPC `Aborted` и могут заново выполнить цикл
  read-modify-write.
- Идемпотентное создание: повторы с одинаковым `idempotency_key`
  возвращают закешированный booking id. Другое тело запроса с тем же
  ключом возвращает `AlreadyExists`, чтобы вызывающая сторона не могла
  «затереть» предыдущий запрос.

## Схема

Восемь миграций в `migrations/`:

| # | Объект | Назначение |
|---|--------|------------|
| 001 | `booking` | состояние агрегата |
| 002 | `booking_offer_snapshot` | замороженный snapshot предложения на момент создания |
| 003 | `booking_status_history` | аудит-лог переходов, читается в `GetBooking` |
| 004 | `booking_projection` | денормализованная модель чтения, удобная для list-запросов |
| 005 | `idempotency_key` | хеш запроса + booking id, уникален по ключу |
| 006 | `outbox_message` | строки событий, вычитываемые outbox-worker'ом |
| 007 | `ALTER booking` | столбец `version` для оптимистической блокировки |
| 008 | `ALTER outbox_message` | `attempt_count`, `locked_at`, `next_retry_at` для повторов и `DLQ` |

`migrations/embed.go` отдает `embed.FS`, который при старте использует
`postgres.RunMigrations`.

## Сценарии отказа

- БД недоступна при старте: `db.PingContext` падает по 5-секундному
  preflight-таймауту, процесс завершается с ненулевым кодом.
- БД отваливается на ходу: текущие RPC возвращают `Internal`; `readyz`
  переключается в 503, как только `Ping` падает.
- NATS недоступен: outbox-worker логирует и повторяет; projector-worker
  блокируется на resubscribe. `readyz` сообщает, что nats не готов.
  Записи продолжают проходить; события накапливаются в outbox.
- Падение между коммитом и публикацией: следующий цикл outbox'а
  подхватывает строку и публикует ее. Дубликат публикации возможен только
  при потере ack от NATS; consumer'ы должны быть идемпотентны (projector
  делает upsert по booking id).
- Параллельные обновления: детектируются по несовпадению версии и
  отдаются как `Aborted`.
