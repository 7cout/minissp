# DBeaver

## Что это

Универсальный GUI-клиент для работы с базами данных. Бесплатная Community Edition 
поддерживает большинство реляционных СУБД.

## Зачем в проекте MiniSSP

- Администрировать и вести разработку PostgreSQL и ClickHouse в GUI,
  а не терминале

## Подключения для MiniSSP

### PostgreSQL
- Host: localhost
- Port: 5432 (TCP)
- Database: minissp
- User: minissp (из .env)
- Password: POSTGRES_PASSWORD (из .env)

### ClickHouse
- Host: localhost 
- Port: 8123 (HTTP)
- User: default (из .env)
- Password: CLICKHOUSE_PASSWORD (из .env)

## Ограничения бесплатной версии

Community Edition не поддерживает:
- Redis
- Kafka

Для Redis и kafka поищу свои стандартные решения