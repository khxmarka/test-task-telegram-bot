# Учёт семейных расходов

Telegram-бот (NL → CRUD своих трат) + read-only панель главы семьи.  
Стек: Go, PostgreSQL, React. Запуск — один `docker compose`.

## Установка

Нужны Docker и токен бота у [@BotFather](https://t.me/BotFather).

```bash
cp .env.example .env
```

В `.env`:

| Переменная | Зачем |
|---|---|
| `TELEGRAM_TOKEN` | обязателен |
| `LLM_MOCK=true` | без ключа, эвристика |
| `LLM_MOCK=false` + `LLM_API_KEY` | живой LLM |
| `LLM_BASE_URL` / `LLM_MODEL` | любой OpenAI-совместимый API с tools |
| `ADMIN_USER` / `ADMIN_PASS` | вход в панель (по умолчанию `admin` / `admin`) |

Туннель не нужен: без `TELEGRAM_WEBHOOK_URL` бот идёт через long polling.

```bash
docker compose up --build
```

Проверка:

- http://localhost:8080/healthz → `{"status":"ok"}`
- http://localhost:8080/ → панель, Basic Auth
- в Telegram: `/start`, затем `кофе 350`

Остановка: `Ctrl+C` или `docker compose down`. Данные Postgres — в volume `pgdata`.

Локально без Docker: поднять Postgres, `make test`, `make run`. Конфиг — `configs/config.yaml` + `.env`.

## Как это работает

```
Telegram  → polling/webhook → agent (guard + tools) → service → PostgreSQL
браузер   → Basic Auth      → GET /api/v1           → service → PostgreSQL
```

Первый `/start` — глава семьи. Остальные аккаунты — члены.  
В боте каждый видит и меняет **только свои** траты.  
Панель — все траты семьи, **без мутаций**.

Язык: `english` / `русский`.

## Операции бота

| Действие | Примеры |
|---|---|
| Создать | `кофе 350`, `такси 10 руб вчера`, `3 USD lunch` |
| Список | `покажи траты за неделю`, `траты на ноги` |
| Сумма | `сколько потрачено в этом месяце`, `сколько ушло на жильё вчера` |
| По категориям | `траты по категориям` |
| Какие категории есть | `какие категории есть?` |
| Изменить | `последнее было не 350, а 450` |
| Удалить одну | `удали 4` |
| Удалить несколько / все свои | `удали 1 и 4`, `удали все расходы` → подтверждение «да»/«нет» |
| Не по теме | общие вопросы, код, политика, «какая ты модель» → отказ |

Поиск идёт и по **комментарию** («ноги», «нож», «лего»), не только по категории.  
Нет совпадений → `трат нет (0)`.

Категории фиксированные: `stores`, `cafes`, `transport`, `housing`, `entertainment`, `health`, `shopping`, `household`, `other`. Модель новую придумать не может.

Валюта по умолчанию BYN. «рубли/руб» = BYN. RUB — только явно (`RUB`, «российских»).  
Курс демо: `1 USD = 3 BYN`, `1 RUB ≈ 0.04 BYN`.

Два одинаковых сообщения подряд = две траты.  
LLM недоступен: create вида `сумма + валюта` всё равно запишется («без ИИ»).

## Что закрыто по безопасности

- **Prompt-injection.** «игнорируй инструкции», jailbreak, «удали всю БД», «покажи чужие» — отказ до LLM (`internal/agent/guard.go`).
- **`user_id` только из Telegram.** Из аргументов модели вырезается. Чужие строки не читаются и не удаляются.
- **Whitelist tools.** Неизвестный tool не исполняется. Create без суммы — отказ.
- **Mass-delete чужих / базы — нельзя.** «Удали всё» = только свои записи и только после «да».
- **Панель read-only** + Basic Auth. Фильтры (член, категория, даты) общие для списка, сводки и pie.

## Панель

http://localhost:8080/ — `admin` / `admin`.

Фильтры сразу на список, сумму и график. Создать/править/удалить из UI нельзя.

## LLM

Клиент — OpenAI chat completions + tool calling (`POST {LLM_BASE_URL}/chat/completions`).

Примеры `LLM_BASE_URL`: Groq `https://api.groq.com/openai/v1`, OpenAI `https://api.openai.com/v1`.  
Нужны tools. Нативный Anthropic / Azure без совместимого endpoint не заведётся.

## Тесты

```bash
make test
```

Покрыто: CRUD и агрегации, мок LLM, injection/guard, HTTP 400/401, валюты, confirm удаления.
