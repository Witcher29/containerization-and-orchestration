## Часть 0

### Создание кластера для сервиса api

Написан сервис с необходимыми эндпоинтами `/health`, `/fail`, `/slow` (спит 3 секунды) и `/load` (делает 20 запросов в себя).
Также написан Makefile и скрипты для создания кластеров Kubertenics.
Для работы с кластерами используются следующие команды:
- `make cluster-up` - создает кластеры
- `make cluster-down` - удалить кластер
- `make deploy` - запускает кластеры
- `make port-forward` - пробросить порт 8080
- `make build` - собрать Docker-образ
- `make port-forward:` - собрать образ и загрузить в kind
- `make image-load: build` - развернуть Helm-чарт
- `make сlean` - удалить все

<img width="1767" height="891" alt="image" src="https://github.com/user-attachments/assets/8f79f555-ef5a-41fe-a935-4976aef94449" />

<img width="1246" height="168" alt="image" src="https://github.com/user-attachments/assets/066cd801-dd9f-400b-8662-fbccd43ff3fe" />

## Часть 1

### Метрики (Prometheus + Grafana)

<img width="1846" height="783" alt="image" src="https://github.com/user-attachments/assets/73dea969-c50c-4971-abb8-b69a255be9a3" />

Запустим Prometheus:

<img width="1106" height="102" alt="image" src="https://github.com/user-attachments/assets/f6892e63-f248-4dfc-bae5-e9ce994f5c0e" />

Запустим Grafana:

<img width="809" height="121" alt="image" src="https://github.com/user-attachments/assets/c5740160-021c-4470-82d4-a90c29349efa" />


## Часть 2

Для сбора и просмотра логов в Kubernetes был развёрнут стек Loki + Alloy. Loki используется как хранилище логов, а Alloy работает как агент сбора логов на узлах кластера.

Alloy был запущен в виде DaemonSet, поэтому его экземпляр работает на каждом узле Kubernetes-кластера. Он обнаруживает логи Kubernetes Pod'ов и отправляет их в Loki.

Loki был настроен в режиме SingleBinary с хранением данных в файловой системе. В Grafana был добавлен Loki как дополнительный источник данных. После этого логи Pod'ов стали доступны непосредственно в интерфейсе Grafana через раздел Explore.

Для проверки работы логирования был использован endpoint /fail приложения. При обращении к нему приложение возвращает ошибку и записывает соответствующее событие в JSON-лог.

Например, для генерации нескольких ошибок использовалась команда:

for i in {1..10}; do
  curl -s http://127.0.0.1:8080/fail >/dev/null
done

<img width="688" height="373" alt="image" src="https://github.com/user-attachments/assets/b9448a85-bc4d-4c5b-9259-c7303d215d2b" />

## Часть 4. Alertmanager и Karma
Для автоматического контроля состояния приложения был настроен Alertmanager поверх Prometheus. В Prometheus были добавлены три критических правила для API-сервиса.

Первое правило отслеживает долю ошибок. Если доля ошибочных запросов превышает 20% в течение двух минут, срабатывает alert ApiServiceHighErrorRate.

Второе правило отслеживает задержку ответов. Если p95 времени ответа превышает одну секунду в течение двух минут, срабатывает ApiServiceHighP95Latency.

Третье правило проверяет доступность самого сервиса. Если Prometheus не может получить метрики API в течение одной минуты, срабатывает ApiServiceDown.

Правила имеют уровень:

severity: critical

После создания PrometheusRule все три правила были проверены в Prometheus и Alertmanager.

## Проверка высокого количества ошибок

Для генерации ошибок использовался endpoint /fail:

for i in {1..40}; do
  curl -s http://127.0.0.1:8080/fail >/dev/null
done

Сначала alert перешёл в статус pending.

<img width="974" height="426" alt="image" src="https://github.com/user-attachments/assets/0c5a15d8-e60b-43d5-a37e-fc8fd0f050e8" />

После двух минут alert перешёл в статус firing

После накопления необходимого количества запросов и истечения времени for: 2m в Alertmanager появился alert ApiServiceHighErrorRate.

<img width="974" height="364" alt="image" src="https://github.com/user-attachments/assets/fab62f40-41ce-49a4-b5a2-e38d895750f7" />

Как раз в этот момент наблюдался высокий процент запросов с ошибками

<img width="706" height="251" alt="image" src="https://github.com/user-attachments/assets/9478bd2d-a175-4fc3-969d-038268bad096" />




## Проверка высокого p95

Для увеличения времени ответа использовался endpoint /slow:

for i in {1..30}; do
  curl -s http://127.0.0.1:8080/slow >/dev/null
done
После этого в promethues alert отображался в статусе firing

<img width="974" height="328" alt="image" src="https://github.com/user-attachments/assets/f778f6c9-de54-4c57-a940-814e8b6a2ec1" />

После этого в Alertmanager сработал alert ApiServiceHighP95Latency.

Для более удобного просмотра алертов поверх Alertmanager был установлен Karma. Karma подключена непосредственно к сервису Alertmanager внутри Kubernetes-кластера.

После запуска Karma все активные алерты стали доступны в одном веб-интерфейсе. В интерфейсе можно фильтровать и группировать алерты, например по имени alert'а, severity и сервису.

В Karma были проверены сработавшие правила:

ApiServiceHighErrorRate
ApiServiceHighP95Latency

<img width="859" height="554" alt="image" src="https://github.com/user-attachments/assets/f32b4dda-666b-4bd5-a13b-96b34e156184" />


Karma оказалась удобнее стандартного интерфейса Alertmanager для одновременного просмотра нескольких алертов, поскольку позволяет быстро отфильтровать нужные события и видеть их состояние в одном окне.


