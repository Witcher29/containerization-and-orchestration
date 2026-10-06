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

Для сбора метрик использован стек kube-prometheus-stack, установленный через Helm в namespace observability. В него входят Prometheus (хранилище метрик), Grafana (визуализация), Alertmanager (алерты), Prometheus Operator (управление ServiceMonitor'ами), kube-state-metrics и node-exporter.

<img width="820" height="398" alt="image" src="https://github.com/user-attachments/assets/eab680fa-0872-4f52-877f-b3f724d3f2ea" />


Сервис api отдаёт метрики на /metrics в формате Prometheus. Набор — RED: счётчик запросов http_requests_total с лейблами method/route/code, счётчик ошибок http_errors_total, гистограмма http_request_duration_seconds.

Чтобы Prometheus узнал, откуда забирать метрики, создан ServiceMonitor — CRD от Prometheus Operator. В нём указано: выбирать Service по лейблу app: api-service-api-service, скрейпить порт с именем http по пути /metrics каждые 15 секунд.
Лейбл release: kube-prom нужен, чтобы оператор вообще увидел этот ServiceMonitor. Сначала ServiceMonitor попал в статус dropped, потому что у порта Service не было имени. После добавления name: http в service.yaml и переустановки Helm-чарта он перешёл в active, а в Prometheus появился таргет api-service в состоянии up.

<img width="1836" height="998" alt="image" src="https://github.com/user-attachments/assets/59c169a7-2122-44ed-9acf-d0b5afaeaeca" />
В Grafana построен дашборд из трёх панелей:

RPS — sum(rate(http_requests_total[5m])) by (route) — сколько запросов в секунду приходит на каждый эндпоинт.

<img width="1836" height="998" alt="image" src="https://github.com/user-attachments/assets/5601aa3e-f1c5-4e3f-8033-086ee716ab6c" />


Error Rate — sum(rate(http_requests_total{code=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) — доля ошибок 5xx.

<img width="1846" height="706" alt="image" src="https://github.com/user-attachments/assets/d58e0b12-65c8-40a0-ad6d-757520614d63" />

p95 Latency — histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le, route)) — 95-й процентиль времени ответа.

<img width="1846" height="816" alt="image" src="https://github.com/user-attachments/assets/206b2529-2358-4c5f-b761-374a1b4b48b3" />

Проверка: при вызове /load и /fail растёт RPS и Error Rate (до 100% при серии /fail), при вызове /slow p95 поднимается до ~5 секунд. 

<img width="651" height="177" alt="image" src="https://github.com/user-attachments/assets/226c330d-010f-41be-b3bb-4480eac9b2b4" />

<img width="1859" height="1006" alt="image" src="https://github.com/user-attachments/assets/5c419c0b-eaf4-4d82-a91c-a195c3d771ea" />

## Часть 2. Alertmanager и Karma

Для сбора и просмотра логов в Kubernetes был развёрнут стек Loki + Alloy. Loki используется как хранилище логов, а Alloy работает как агент сбора логов на узлах кластера.

Alloy был запущен в виде DaemonSet, поэтому его экземпляр работает на каждом узле Kubernetes-кластера. Он обнаруживает логи Kubernetes Pod'ов и отправляет их в Loki.

Loki был настроен в режиме SingleBinary с хранением данных в файловой системе. В Grafana был добавлен Loki как дополнительный источник данных. После этого логи Pod'ов стали доступны непосредственно в интерфейсе Grafana через раздел Explore.

Для проверки работы логирования был использован endpoint /fail приложения. При обращении к нему приложение возвращает ошибку и записывает соответствующее событие в JSON-лог.

Например, для генерации нескольких ошибок использовалась команда:

for i in {1..10}; do
  curl -s http://127.0.0.1:8080/fail >/dev/null
done

<img width="688" height="373" alt="image" src="https://github.com/user-attachments/assets/b9448a85-bc4d-4c5b-9259-c7303d215d2b" />

## Часть 3

### Трейсы

Сервис api инструментирован OpenTelemetry SDK — пакеты go.opentelemetry.io/otel/* подключены в go.mod. SDK создаёт root-спан на каждый входящий HTTP-запрос, а в отдельных обработчиках добавляются вложенные спаны.

Спаны экспортируются по OTLP HTTP в Jaeger на адрес http://observability-jaeger.monitoring:4318. Из необычных проблем - в файле values.yaml endpoint задаётся без схемы http://, потому что SDK использует otlptracehttp.WithEndpoint, который ожидает только host:port и добавляет схему сам. Пример работы ниже:

<img width="1852" height="1006" alt="image" src="https://github.com/user-attachments/assets/c8f32cd2-f701-43aa-b42a-4d1406c54345" />

Вышеуказанные запросы:

<img width="924" height="126" alt="image" src="https://github.com/user-attachments/assets/28aaa89c-d214-4f12-8eab-d5cd1f01a818" />

В /slow создаётся вложенный спан slow-op через tracer.Start(ctx, "slow-op"). Он становится дочерним для root-спана GET /slow, потому что в контексте запроса уже лежит родительский спан. Внутри slow-op выполняется time.Sleep — имитация медленной операции. В Jaeger waterfall видно, что всё время запроса ушло именно в slow-op — так надо для более удобной локализации задержки.

<img width="1857" height="831" alt="image" src="https://github.com/user-attachments/assets/56610ca5-2ad3-4654-bbe8-af9d1d603a94" />

В /fail спан помечается как ошибочный: span.SetStatus(codes.Error, ...) и span.RecordError(...). В Jaeger он отображается красным, а в тегах спана видны error=true, otel.status_code=ERROR, http.status_code=500.

<img width="1851" height="730" alt="image" src="https://github.com/user-attachments/assets/2cf2c4c7-8bd9-46ec-a5a2-6039b28041cb" />

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



