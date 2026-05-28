# gRPC API

Сервис: `booking.v1.BookingService`. Proto-исходники лежат в
`api/proto/booking/v1/`, сгенерированный Go-код — в `gen/go/booking/v1/`.

Все RPC — унарные. Сервер навешивает дедлайн (`RPC_TIMEOUT`, по умолчанию
30 секунд) через unary-interceptor.

## Общие типы

`Money` — amount как десятичная строка («199.99»), валюта по ISO 4217. На
проводе amount передается строкой для сохранения точности; внутри сервера
используется int64 в минимальных единицах.

`OfferSnapshot` — `{offer_id, hotel_id, room_type, price_per_night}`.
Snapshot замораживается на момент создания и больше не перечитывается.

`BookingStatus` — `BOOKING_STATUS_PENDING`, `APPROVED`, `REJECTED`,
`CANCELLED`. `UNSPECIFIED` означает «без фильтра» в `ListBookings`.

## RPC: CreateBooking

`rpc CreateBooking(CreateBookingRequest) returns (CreateBookingResponse)`

Поля запроса:

| поле | тип | обязательное | примечания |
|------|-----|--------------|------------|
| `idempotency_key` | string | да | генерируется клиентом; тот же ключ + тот же payload возвращают то же бронирование |
| `guest_id` | string | да |  |
| `offer` | `OfferSnapshot` | да | фиксируется на момент создания |
| `check_in` | `Timestamp` | да | обрезается до даты |
| `check_out` | `Timestamp` | да | должно быть позже `check_in` |

Ответ: `Booking`.

Коды ошибок:

- `InvalidArgument` — отсутствует обязательное поле, некорректный money
  или `check_out <= check_in`.
- `AlreadyExists` — `idempotency_key` переиспользован с другим payload'ом.
- `Aborted` — конфликт записи на уровне хранилища (для create — редкость).
- `Internal` — отказ db/io.

## RPC: GetBooking

`rpc GetBooking(GetBookingRequest) returns (GetBookingResponse)`

Строго согласованное чтение из модели записи.

Запрос:

| поле | тип | обязательное |
|------|-----|--------------|
| `booking_id` | string | да |

Ответ: `Booking` плюс срез `history` из `StatusHistoryEntry`
(`{old_status, new_status, reason, changed_at}`) в хронологическом порядке.

Коды ошибок:

- `InvalidArgument` — пустой `booking_id`.
- `NotFound` — бронирования с таким id нет.
- `Internal` — отказ БД.

## RPC: ListBookings

`rpc ListBookings(ListBookingsRequest) returns (ListBookingsResponse)`

Читает из «CQRS»-проекции. Согласован в конечном счете: только что
созданное бронирование может не появиться, пока projector не применит
его событие.

Запрос:

| поле | тип | обязательное | примечания |
|------|-----|--------------|------------|
| `guest_id` | string | да | фильтр по гостю |
| `status_filter` | `BookingStatus` | нет | `UNSPECIFIED` = все статусы |
| `check_in_from` | `Timestamp` | нет | включительная нижняя граница по `check_in` |
| `check_in_to` | `Timestamp` | нет | исключительная верхняя граница по `check_in` |
| `page_size` | int32 | нет | по умолчанию 20, максимум 100 |
| `page_token` | string | нет | непрозрачный offset, возвращенный предыдущим вызовом |

Ответ: список `BookingSummary` плюс `next_page_token`. Пустой токен
означает, что страниц больше нет.

Коды ошибок:

- `InvalidArgument` — пустой `guest_id` или нераспарсиваемый `page_token`.
- `Internal` — отказ БД.

## RPC: CancelBooking

`rpc CancelBooking(CancelBookingRequest) returns (CancelBookingResponse)`

Переводит бронирование в `CANCELLED`. Разрешено из `PENDING`.

Запрос:

| поле | тип | обязательное |
|------|-----|--------------|
| `booking_id` | string | да |

Ответ: `Booking`.

Коды ошибок:

- `InvalidArgument` — пустой `booking_id`.
- `NotFound` — бронирования с таким id нет.
- `FailedPrecondition` — текущий статус терминальный
  (`APPROVED`/`REJECTED`/`CANCELLED`) или переход недопустим по иным
  причинам.
- `Aborted` — конфликт оптимистической блокировки; повторить цикл
  read-modify-write.
- `Internal` — отказ БД.

## RPC: ApproveBooking

`rpc ApproveBooking(ApproveBookingRequest) returns (ApproveBookingResponse)`

Переводит бронирование в `APPROVED`. Разрешено из `PENDING`.

Запрос:

| поле | тип | обязательное |
|------|-----|--------------|
| `booking_id` | string | да |

Ответ: `Booking`.

Коды ошибок: те же, что у `CancelBooking`.

## RPC: RejectBooking

`rpc RejectBooking(RejectBookingRequest) returns (RejectBookingResponse)`

Переводит бронирование в `REJECTED`. Разрешено из `PENDING`. Причина
сохраняется в истории статусов.

Запрос:

| поле | тип | обязательное |
|------|-----|--------------|
| `booking_id` | string | да |
| `reason` | string | нет |

Ответ: `Booking`.

Коды ошибок: те же, что у `CancelBooking`.

## События (out-of-band)

Доменные события публикуются в стрим NATS JetStream `booking-events`.
Subject совпадает с именем события. Payload — JSON; `key` (id сообщения
NATS) — это booking id.

| Subject | Payload |
|---------|---------|
| `booking.created` | `{booking_id, guest_id, hotel_id, room_type, check_in, check_out, total{amount,currency}, timestamp}` |
| `booking.cancelled` | `{booking_id, old_status, new_status, timestamp}` |
| `booking.approved` | `{booking_id, old_status, new_status, timestamp}` |
| `booking.rejected` | `{booking_id, old_status, new_status, reason, timestamp}` |

Доставка — at-least-once через «transactional outbox»; consumer'ы
обязаны быть идемпотентными.
