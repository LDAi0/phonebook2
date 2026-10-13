# Phonebook backend

Бэкенд телефонной книги на Go: REST API, PostgreSQL, Kafka, миграции, Docker.

## Архитектура (3 слоя)

```
HTTP-клиент
    │
    ▼
┌──────────────────────────────┐
│ Presentation  transport/httpapi │  HTTP ↔ DTO, коды ответов, middleware
└──────────────┬───────────────┘
               ▼
┌──────────────────────────────┐
│ Business      service         │  валидация, нормализация, оркестрация, события
└───────┬──────────────┬───────┘
        ▼              ▼
┌───────────────┐  ┌──────────────────┐
│ Data          │  │ Messaging adapter │
│ repository    │  │ events (Kafka)    │
│ (pgx/Postgres)│  │                   │
└───────────────┘  └──────────────────┘
```

* Зависимости направлены только вниз; слой `service` знает лишь интерфейсы
  (`ContactRepository`, `EventPublisher`), поэтому тестируется без БД и Kafka.
* `domain` — общие сущности и ошибки, ничего не импортирует.
* Два бинарника: `cmd/server` (API, продюсер событий) и `cmd/consumer`
  (читает топик и пишет аудит в таблицу `contact_events`, идемпотентно).

```
cmd/server        точка входа API
cmd/consumer      точка входа Kafka-консьюмера
internal/config   конфигурация из env
internal/domain   сущности, ошибки, события
internal/service  бизнес-слой (+ unit-тесты)
internal/repository  слой данных (pgx)
internal/events   Kafka producer / consumer
internal/transport/httpapi  HTTP-слой
migrations        SQL-миграции (golang-migrate)
```

## Запуск

```bash
go mod tidy          # один раз: сгенерирует go.sum
make up              # postgres + миграции + kafka + api + consumer
curl localhost:8080/healthz
```

Порядок старта в compose: `postgres → migrate → (api, consumer)`,
`kafka → kafka-init (создаёт топик, 3 партиции) → (api, consumer)`.

Остановить: `make down` (данные сохраняются в volume `pgdata`; `docker compose down -v` — стереть).

## API

| Метод  | Путь                    | Описание                                   |
|--------|-------------------------|--------------------------------------------|
| POST   | `/api/v1/contacts`      | создать контакт                            |
| GET    | `/api/v1/contacts`      | список, `?q=поиск&limit=20&offset=0`       |
| GET    | `/api/v1/contacts/{id}` | получить контакт                           |
| PUT    | `/api/v1/contacts/{id}` | обновить контакт                           |
| DELETE | `/api/v1/contacts/{id}` | удалить контакт                            |
| GET    | `/healthz`, `/readyz`   | liveness / readiness (проверяет БД)        |

Телефон нормализуется (пробелы, `-`, `()` убираются), формат `+?[0-9]{7,15}`, уникален.
Ошибки: `422` валидация, `404` не найден, `409` телефон уже есть.

```bash
curl -X POST localhost:8080/api/v1/contacts -H 'Content-Type: application/json' \
  -d '{"first_name":"Ada","last_name":"Lovelace","phone":"+372 5555 1234","email":"ada@example.com"}'

curl 'localhost:8080/api/v1/contacts?q=ada'
curl -X PUT localhost:8080/api/v1/contacts/1 -H 'Content-Type: application/json' \
  -d '{"first_name":"Ada","last_name":"King","phone":"+37255551234"}'
curl -X DELETE localhost:8080/api/v1/contacts/1
```

## Kafka

Каждое изменение публикуется в топик `phonebook.contacts`
(`contact.created / updated / deleted`), ключ сообщения — id контакта,
поэтому события одного контакта идут по порядку внутри партиции.

```bash
# смотреть события вживую
docker compose exec kafka /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server kafka:9092 --topic phonebook.contacts --from-beginning

# аудит, записанный консьюмером
docker compose exec postgres psql -U phonebook -c \
  "select event_type, contact_id, occurred_at from contact_events order by id"
```

## Тесты

```bash
make test
```

## Осознанные компромиссы и что улучшить дальше

* Публикация в Kafka — best-effort после коммита в БД: при падении Kafka запрос
  не падает, ошибка логируется. Для гарантированной доставки — transactional outbox.
* Консьюмер даёт at-least-once; дубликаты отсекаются по `event_id` (UNIQUE).
* Нет аутентификации; для продакшена добавить JWT/API-key middleware, метрики (Prometheus), tracing.
