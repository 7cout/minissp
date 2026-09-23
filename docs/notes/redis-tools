# Redis: GUI и CLI

Взаимодействие с Redis идёт двумя способами: через GUI (Tiny RDM) и через 
консольный клиент (redis-cli). Они дополняют друг друга, как Kafka UI и CLI.

## Tiny RDM

### Что это

Бесплатный GUI-клиент для Redis. Написан на Go + Wails.

### Подключение к проекту

- Name: minissp-redis
- Host: 127.0.0.1
- Port: 6379
- Username: (пусто)
- Password: (пусто)
- DB: 0

### Что умеет

- Дерево ключей слева, фильтр по паттерну (`slot:*`)
- Просмотр всех типов: string, hash, list, set, zset, stream
- Редактирование значений в UI
- Колонка TTL у каждого ключа
- Monitor — поток команд в реальном времени
- Встроенный CLI

## redis-cli

### Что это

Официальный консольный клиент Redis. Уже есть внутри контейнера, 
устанавливать не нужно.

### Основные команды

Интерактивная сессия:

    docker exec -it minissp-redis redis-cli

Одна команда:

    docker exec -it minissp-redis redis-cli PING
    docker exec -it minissp-redis redis-cli SET greeting "hello"
    docker exec -it minissp-redis redis-cli GET greeting
    docker exec -it minissp-redis redis-cli TTL greeting
    docker exec -it minissp-redis redis-cli DEL greeting

Работа с хэшем:

    docker exec -it minissp-redis redis-cli HSET slot:test width 320 height 50
    docker exec -it minissp-redis redis-cli HGETALL slot:test
    docker exec -it minissp-redis redis-cli EXPIRE slot:test 300

Список всех ключей:

    docker exec -it minissp-redis redis-cli KEYS "*"

## Когда что использовать

| Задача | Инструмент |
|---|---|
| Проверить, что Redis жив | CLI (`PING`) |
| Установить / прочитать одно значение | CLI |
| Посмотреть все ключи | GUI |
| Изучить содержимое хэша | GUI |
| Проверить TTL у многих ключей сразу | GUI |
| Посмотреть поток команд | GUI (Monitor) |
| Скрипт / автоматизация | CLI |
| Быстро удалить ключ | CLI или GUI |

Оба смотрят на один и тот же Redis. Изменения через CLI сразу видны в GUI, 
и наоборот.

## Ссылки

- Tiny RDM: https://github.com/tiny-craft/tiny-rdm
- Redis CLI docs: https://redis.io/docs/connect/cli/