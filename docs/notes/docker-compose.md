# Docker Compose

## Что это

Файл, где я описываю все контейнеры проекта.
Одна команда `docker compose up -d` вместо кучи `docker run`

## YAML

- Отступы — пробелы, не табы. Обычно 2 пробела.
- Списки — через дефис: `- "5432:5432"`.
- Кавычки — там, где значение может быть прочитано не так 
  (`"5432:5432"` без кавычек YAML примет за время).

## Структура

    name: minissp

    services:
      postgres:
        ...
      kafka:
        ...

    volumes:
      postgres-data:

## Что пишу в сервисе

- `image` — образ с версией (не `latest`).
- `container_name` — имя контейнера.
- `restart: unless-stopped` — перезапускать, если упал.
- `environment` — переменные из `.env`.
- `ports` — `"хост:контейнер"`.
- `volumes` — куда монтировать данные.
- `healthcheck` — проверка, что сервис жив.
- `depends_on` — ждать другой сервис.

## Порты

    ports:
      - "8080:8080"
        ↑     ↑
        хост  контейнер

- Правый — не трогаю, его диктует сам сервис.
- Левый — меняю, если занят.
- `address already in use` — конфликт на хосте, не в контейнерах.

## Переменные

    environment:
      POSTGRES_USER: ${POSTGRES_USER}

Берётся из `.env`. Сам `.env` не коммичу, только `.env.example`.

## Как контейнеры видят друг друга

**Имя сервиса = hostname внутри Docker-сети.**

    KAFKA_CLUSTERS_0_BOOTSTRAPSERVERS: kafka:9092

Не `localhost`, а имя сервиса `kafka`.

Проверить: `docker exec -it <контейнер> cat /etc/hosts`.

## localhost vs имя сервиса

- В контейнере к другому контейнеру — `kafka:9092`.
- В контейнере к хосту — `host.docker.internal`.
- На Windows к контейнеру — `localhost:9092`.

## Volumes

    services:
      postgres:
        volumes:
          - postgres-data:/var/lib/postgresql/data   # ссылка
    volumes:
      postgres-data:                                  # объявление

Нужны обе секции.

| Команда | Данные |
|---|---|
| `down` | Остаются |
| `down -v` | Пропадают |

## Healthcheck

    healthcheck:
      test: ["CMD", "..."]
      interval: 10s
      timeout: 5s
      retries: 5

`CMD` — напрямую, `CMD-SHELL` — через shell.

## Версии образов

Только конкретная версия, не `latest`. Чтобы завтра было то же самое, 
что сегодня.

## Команды

- `docker compose up -d` — запустить.
- `docker compose down` — остановить.
- `docker compose down -v` — остановить + удалить данные.
- `docker compose ps` — статус.
- `docker compose logs -f <сервис>` — логи.
- `docker compose config` — проверить файл.
- `docker compose exec <сервис> sh` — зайти внутрь.

## Что чаще всего ломается

- Неверный отступ → `mapping values are not allowed`.
- Нет `.env` → `environment variable X is not set`.
- Порт занят → `bind: address already in use`.
- Опечатка в образе → `pull access denied`.