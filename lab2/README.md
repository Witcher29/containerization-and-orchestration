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

