# HTTP API

База: корень сервера, по умолчанию `http://localhost:8080`.  
Формат: JSON, кодировка UTF-8.  
Заголовок запроса с телом: `Content-Type: application/json`.

---

## Аутентификация

Все маршруты API (кроме `GET /health`) обёрнуты в middleware **`BearerOwner`**.

### Заголовок `Authorization`

```
Authorization: Bearer <owner_uuid>
```

`<owner_uuid>` — валидный UUID владельца данных (пользователь/устройство). Токен **не** JWT: это сам идентификатор владельца. Неверный формат → `401 UNAUTHORIZED`.

### Заголовок `X-Device-Id` (необязательно)

Строка для аудита «кто создал» сущность. Если передан, сохраняется как `created_by` там, где это поддерживается.

---

## Проверка живости (без авторизации)

### `GET /health`

Проверка доступности приложения и пинга Postgres.

| Ответ | Тело |
|--------|------|
| `200 OK` | `ok\n` (текст) |
| `503 Service Unavailable` | `db: unavailable\n` |

---

## Ошибки

Успешные ответы с телом — JSON. Ошибки — тоже JSON:

```json
{
  "code": "NOT_FOUND",
  "message": "человекочитаемое описание"
}
```

Типичные `code`:

| `code` | HTTP | Когда |
|--------|------|--------|
| `UNAUTHORIZED` | 401 | Нет или неверный `Authorization` |
| `BAD_REQUEST` | 400 | Невалидный JSON, UUID, поля |
| `NOT_FOUND` | 404 | Сущность не найдена или не принадлежит владельцу |
| `CONFLICT` | 409 | Конфликт бизнес-правил |
| `INTERNAL` | 500 | Внутренняя ошибка (детали не раскрываются) |

Тела запросов декодируются с **`DisallowUnknownFields`**: лишние поля в JSON → `400 BAD_REQUEST`.

---

## Товары

### `POST /goods`

Создание или обновление товара по `id` (upsert).

**Тело:**

| Поле | Тип | Обязательно |
|------|-----|-------------|
| `id` | UUID | да |
| `name` | string | да |

**Ответ `200`:** объект товара (см. ниже).

---

### `GET /goods?q=...`

Поиск товаров владельца. `q` — опциональная строка поиска.

**Ответ `200`:** массив сниппетов:

```json
[{ "id": "…", "name": "Молоко" }]
```

---

### `GET /goods/{id}`

Канонический товар по `id`.

**Ответ `200`:** объект товара:

| Поле | Тип |
|------|-----|
| `id` | UUID |
| `owner_id` | UUID |
| `name` | string |
| `normalized_name` | string |
| `merged_into` | UUID или отсутствует |
| `created_at` | RFC3339 |
| `updated_at` | RFC3339 |

---

### `POST /goods/merge`

Объединение двух товаров.

**Тело:**

| Поле | Тип |
|------|-----|
| `source_good_id` | UUID |
| `target_good_id` | UUID |

**Ответ `204 No Content`** при успехе.

---

### `GET /goods/{id}/merge-candidates?q=...`

Кандидаты на слияние с товаром `{id}`. Параметр `q` опционален.

**Ответ `200`:**

```json
{
  "exact": [{ "id": "…", "name": "…" }],
  "prefix": [],
  "contains": [],
  "others": []
}
```

---

### `GET /goods/{id}/offers`

Предложения (офферы) по товару с последней известной ценой по каждому офферу.

**Ответ `200`:** массив:

```json
[
  {
    "offer_id": "…",
    "store": { "id": "…", "name": "…" },
    "latest_price": {
      "price": 99.9,
      "pack_size": 1,
      "unit": "pcs"
    }
  }
]
```

Поле `latest_price` может отсутствовать, если цен ещё не было.

---

## Магазины

### `POST /stores`

Upsert магазина.

**Тело:** `id` (UUID), `name` (string).

**Ответ `200`:** как у товара — `id`, `owner_id`, `name`, `normalized_name`, `merged_into`, `created_at`, `updated_at`.

---

### `GET /stores?q=...`

Поиск магазинов.

**Ответ `200`:** `[{ "id", "name" }, …]`.

---

## Офферы и цены

### `POST /offers`

Создание оффера «товар в магазине».

**Тело:**

| Поле | Тип |
|------|-----|
| `id` | UUID |
| `good_id` | UUID |
| `store_id` | UUID |

**Ответ `200`:** `id`, `owner_id`, `good_id`, `store_id`, `created_at`, `updated_at`.

---

### `POST /price-records`

Запись цены по офферу.

**Тело:**

| Поле | Тип | Обязательно |
|------|-----|-------------|
| `id` | UUID | да |
| `offer_id` | UUID | да |
| `price` | number | да |
| `pack_size` | number | нет |
| `unit` | string | нет |
| `recorded_at` | RFC3339 | нет; если не указано — текущее UTC время |

**Ответ `200`:** `id`, `owner_id`, `offer_id`, `price`, `recorded_at`, `created_at`, опционально `pack_size`, `unit`.

---

### `GET /offers/{id}/prices`

История цен оффера.

**Ответ `200`:** массив записей цен (как в ответе `POST /price-records`).

---

### `GET /offers/{id}/price/latest`

Последняя цена оффера.

**Ответ `200`:**

```json
{ "latest_price": null }
```

или

```json
{
  "latest_price": {
    "price": 99.9,
    "pack_size": 1,
    "unit": "kg"
  }
}
```

---

## Списки покупок

### `POST /lists`

Upsert списка.

**Тело:** `id` (UUID), `name` (string).

**Ответ `200`:** `id`, `owner_id`, `name`, `created_at`, `updated_at`.

---

### `GET /lists`

Все списки владельца.

**Ответ `200`:** массив объектов списка (как выше).

---

### `GET /lists/{id}`

Список с позициями.

**Ответ `200`:**

```json
{
  "id": "…",
  "owner_id": "…",
  "name": "На неделю",
  "created_at": "…",
  "updated_at": "…",
  "items": [
    {
      "id": "…",
      "owner_id": "…",
      "list_id": "…",
      "good_id": "…",
      "offer_id": "…",
      "quantity": 2,
      "is_purchased": false,
      "good_name": "Молоко",
      "price_snapshot": 89.9,
      "created_at": "…",
      "updated_at": "…"
    }
  ]
}
```

Поля `offer_id` и `price_snapshot` могут отсутствовать, если `null`.

---

### `POST /list-items`

Добавление позиции в список.

**Тело:**

| Поле | Тип | Обязательно |
|------|-----|-------------|
| `id` | UUID | да |
| `list_id` | UUID | да |
| `good_id` | UUID | да |
| `offer_id` | UUID | нет |
| `quantity` | number | да |
| `price_snapshot` | number | нет |

**Ответ `200`:** объект позиции (включая `good_name` после обогащения).

---

### `PATCH /list-items/{id}`

Частичное обновление позиции. Допустимые ключи в теле (любое подмножество):

| Поле | Тип | Примечание |
|------|-----|------------|
| `quantity` | number | |
| `is_purchased` | boolean | |
| `offer_id` | UUID или `null` | `null` снимает привязку к офферу |

**Ответ `200`:** полный объект позиции после обновления.

---

## Внешние идентификаторы товара

### `POST /good-identities`

Привязка внешнего идентификатора к товару (например, ID из внешней системы).

**Тело:**

| Поле | Тип |
|------|-----|
| `good_id` | UUID |
| `external_id` | string (не пустая после trim) |
| `source` | string (не пустая после trim) |

**Ответ `204 No Content`** при успехе.

---

## Postman

Импорт: **Import** → файл [`docs/postman/shoppinglist-back.postman_collection.json`](postman/shoppinglist-back.postman_collection.json).  
В коллекции заданы переменные (`baseUrl`, `ownerId`, UUID сущностей). Запуск всего сценария: **Run collection** (порядок папок рассчитан на happy path, merge — перед негативными тестами).

---

## Пример `curl`

Проверка здоровья:

```bash
curl -s http://localhost:8080/health
```

Запрос к API (подставь свой UUID владельца):

```bash
OWNER="00000000-0000-0000-0000-000000000001"

curl -sS http://localhost:8080/goods \
  -H "Authorization: Bearer $OWNER" \
  -H "Content-Type: application/json"
```

Создание товара:

```bash
curl -sS http://localhost:8080/goods \
  -H "Authorization: Bearer $OWNER" \
  -H "Content-Type: application/json" \
  -H "X-Device-Id: my-phone" \
  -d '{"id":"11111111-1111-1111-1111-111111111111","name":"Хлеб"}'
```

---

## Маршрутизация

Сервер использует паттерны Go 1.22+ (`GET /path`, `POST /path`, path-параметры `{id}`). Порядок регистрации: **`GET /health`** без авторизации; все остальные пути обрабатываются API с **`BearerOwner`**.
