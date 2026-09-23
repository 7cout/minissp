# Kafka: UI и CLI

Взаимодействие с Kafka в проекте идёт двумя способами: через веб-интерфейс 
(Kafka UI) и через консольные утилиты (CLI). Они дополняют друг друга.

## Kafka UI

### Что это

Веб-интерфейс для Kafka. Просмотр топиков, сообщений, consumer groups 
и их отставания (lag). Работает в браузере.

### Доступ

- URL: http://localhost:8080

### Что умеет

- Список всех топиков и их партиций.
- Просмотр сообщений в реальном времени.
- Чтение сообщений с начала (`from beginning`).
- Просмотр consumer groups и их lag.
- Создание и удаление топиков через UI.

## CLI (командная строка)

Утилиты Kafka лежат внутри контейнера в `/opt/kafka/bin/`. 
Запускаются через `docker exec`.

### Список топиков

    docker exec -it minissp-kafka /opt/kafka/bin/kafka-topics.sh \
      --bootstrap-server localhost:9092 --list

### Создать топик

    docker exec -it minissp-kafka /opt/kafka/bin/kafka-topics.sh \
      --bootstrap-server localhost:9092 \
      --create --topic auction-events --partitions 3 --replication-factor 1

### Описание топика

    docker exec -it minissp-kafka /opt/kafka/bin/kafka-topics.sh \
      --bootstrap-server localhost:9092 --describe --topic auction-events

### Отправить сообщение

    echo "test event" | docker exec -i minissp-kafka \
      /opt/kafka/bin/kafka-console-producer.sh \
      --bootstrap-server localhost:9092 --topic auction-events

Флаг `-i` (без `t`) нужен, чтобы передать stdin в контейнер.

### Прочитать сообщения

    docker exec -it minissp-kafka /opt/kafka/bin/kafka-console-consumer.sh \
      --bootstrap-server localhost:9092 --topic auction-events \
      --from-beginning --max-messages 1

### Удалить топик

    docker exec -it minissp-kafka /opt/kafka/bin/kafka-topics.sh \
      --bootstrap-server localhost:9092 --delete --topic auction-events

## Когда что использовать

| Задача | Инструмент |
|---|---|
| Быстро создать топик | CLI |
| Отправить одно тестовое сообщение | CLI |
| Посмотреть поток событий в реальном времени | UI |
| Понять структуру топиков и партиций | UI |
| Отладить consumer group и lag | UI |
| Скрипт / автоматизация | CLI |

Оба инструмента смотрят на одну и ту же Kafka. Изменения через CLI сразу 
видны в UI, и наоборот.