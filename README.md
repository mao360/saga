# Saga — распределённая оркестрация заказов

Демонстрация паттерна **Saga** для распределённой транзакции «создание заказа» между тремя
Go-микросервисами, которые общаются асинхронно через Kafka, с полноценной observability и
нагрузочным тестированием.

- **order** — оркестратор саги: принимает HTTP-заказы, сохраняет заказ и состояние саги, шлёт
  команды, обрабатывает события и компенсации. Использует **outbox-паттерн** для надёжной
  публикации событий.
- **inventory** — резервирует / освобождает товар на складе.
- **payment** — списывает / возвращает средства на счёте.

Каждый сервис владеет своей базой Postgres и применяет миграции при старте. Кирпичики надёжности:
**идемпотентность** (`processed_commands`), **outbox** relay + cleaner (order), трассировка
OpenTelemetry и метрики Prometheus во всех сервисах.

## Архитектура

```
            HTTP
клиент ──────────────▶ order ──(saga.commands)──▶ inventory
                         ▲                          │
                         │                          ▼
                         └──(saga.inventory.events)─┘
                         ▲
                         │
                       payment ◀─(saga.commands)────┘
   order ◀──(saga.payment.events / saga.inventory.events)── inventory / payment
```

Топики Kafka: `saga.commands`, `saga.order.events`, `saga.inventory.events`,
`saga.payment.events`, `saga.dlq`.

## Быстрый старт

```bash
# инфраструктура (Kafka + топики), мониторинг и все три сервиса
make up

# полностью погасить всё
make down
```

Эндпоинты после запуска:

- API заказов — <http://localhost:8080/orders>
- Grafana — <http://localhost:3000> (дашборд **Saga Overview**, включён анонимный просмотр)
- Prometheus — <http://localhost:9090>
- Kafka UI — <http://localhost:8081>

Создать заказ:

```bash
curl -X POST http://localhost:8080/orders \
  -H 'Content-Type: application/json' \
  -d '{"customer":"cust-1","sku":"sku-1","account_id":"acc-1","amount":100,"qty":1}'
```

## Observability и нагрузочное тестирование

Дашборды Grafana **описаны как код** (`monitoring/grafana/`) и обкатаны нагрузкой через
[k6](https://k6.io/) (`loadtest/k6/`). Профиль arrival-rate 50→100→100→0 RPS дал **14 099 заказов
без единой ошибки** (p95 3.65 мс), а лаг консьюмеров Kafka вернулся к нулю после прогона.

![Дашборд Saga Overview](docs/screenshots/saga-overview-full.png)

Полная методология, результаты и честные оговорки про запуск на одной машине:
**[docs/observability.md](docs/observability.md)**.

```bash
make load-prefill      # засеять склад и баланс
make load-arrival      # профиль arrival-rate (~3.5 мин)
# также: load-smoke / load-ramp / load-soak
```

## Тесты

```bash
cd order && go test ./...
```