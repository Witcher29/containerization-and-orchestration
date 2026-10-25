## 1. Цель работы
Развернуть отдельный многонодовый кластер Kubernetes на `kind` с CNI Cilium, проверить связь между Pod и Service, выставить API наружу через `NodePort`, ограничить сетевые соединения с помощью `NetworkPolicy`, проверить разрешённые и запрещённые потоки и продемонстрировать диагностику искусственно внесённой сетевой ошибки. Для метрик используется стек Prometheus/Grafana.

## 2. Архитектура решения
- **Cluster:** `lab5` — отдельный кластер.
- **CNI:** Cilium `v1.20.2` с включённым Hubble (для наблюдаемости).
- **API:** `shop-api`, порт контейнера `8080`, Service `NodePort` `30080`.
- **Worker:** `shop-worker`, инициирует соединения к PostgreSQL.
- **Database:** CloudNativePG, Service записи `shop-postgres-rw`, порт `5432`.
- **Observability:** Prometheus/Grafana; `ServiceMonitor` собирает метрики с API и Cilium.

Схема сетевых потоков:
```text
Host :30080
    |
    v
NodePort Service shop-api :80 -> Pod shop-api :8080
                                      |
                                      | TCP/5432
                                      v
                              shop-postgres-rw

shop-worker ---------------- TCP/5432 ----------------> PostgreSQL
Prometheus (namespace monitoring) -- TCP/8080 -------> shop-api /metrics

## 3. Создание кластера с Calico

В `cluster/kind-calico.yaml` отключён стандартный CNI `kindnet`, потому что CNI-интерфейсы и IPAM должен настраивать Calico.
Для Pod-сети выбран CIDR `192.168.0.0/16`; он совпадает с пулом Calico в манифесте установки. Host-порт `30080` проброшен в control-plane контейнер kind, чтобы NodePort был доступен с хоста.

```bash
make cluster-up
make calico
kubectl config current-context
kubectl get nodes -o wide
kubectl get pods -n kube-system -o wide | grep calico
kubectl get ippools.crd.projectcalico.org
```

Ожидаемое состояние: три узла `Ready`, по одному Pod `calico-node` на каждом узле и работающий `calico-kube-controllers`.

**Если `kind create cluster` завершается ошибкой** с `could not find a log line that matches "Reached target ... Multi-User System"`, скрипт пытается сохранить диагностические логи в `artifacts/kind-logs`. Это ошибка запуска контейнерного узла, а не результат работы NetworkPolicy. Выполните:

```bash
docker info | grep -E 'Server Version|Cgroup Driver|Cgroup Version|Operating System'
kind version
docker ps -a --filter name=lab5-calico
kind export logs --name lab5-calico ./artifacts/kind-logs
```

<img width="815" height="478" alt="image" src="https://github.com/user-attachments/assets/c6b15cea-5c2d-44af-abfa-27a27aed2c17" />

## 4. Развёртывание приложения

```bash
make deploy
kubectl get pods,svc -o wide
kubectl get networkpolicy
```

Скрипт устанавливает CloudNativePG operator, стек наблюдаемости, собирает локальные образы API/worker/batch, загружает их в `lab5-calico` и устанавливает чарт `shop`.

Проверка API с хоста:

```bash
curl -i http://127.0.0.1:30080/health
```

Ожидается `HTTP/1.1 200 OK` и тело `ok`. Создание заказа проверяется запросом:

```bash
curl -i -X POST http://127.0.0.1:30080/order
```

Итоговая проверка:

<img width="927" height="354" alt="image" src="https://github.com/user-attachments/assets/b76667c5-997c-4588-a0ed-99a046e84d96" />

## 5. Политика сетевого доступа

Включён принцип **default deny**: `shop-default-deny` запрещает входящие и исходящие соединения для Pod в namespace приложения. После этого разрешены только необходимые потоки. Отдельная политика разрешает DNS к CoreDNS по UDP/TCP 53.

Для политики используются дополнительные метки `network-role` на Pod. Они отделяют роль в сетевой политике от `app`, который нужен Service для выбора endpoints. Диагностические Pod могут получить роль `api` или `worker`, не становясь endpoint приложения.

| Источник | Назначение | Порт/протокол | Результат | Причина |
|---|---|---:|---|---|
| Host / kind node | `shop-api` | TCP 30080 → 8080 | Разрешено | Внешний доступ к API через NodePort |
| Pod с `network-role=api-client` | `shop-api` | TCP 8080 | Разрешено | Клиентский запрос к API |
| Namespace `monitoring` | `shop-api` | TCP 8080 | Разрешено | Prometheus опрашивает `/metrics` |
| `shop-api` | PostgreSQL | TCP 5432 | Разрешено | Создание/чтение заказов |
| `shop-worker` | PostgreSQL | TCP 5432 | Разрешено | Обработка заказов |
| `cnpg-system` | PostgreSQL | TCP 5432 | Разрешено | Управление и проверки CloudNativePG |
| Обычный Pod без разрешённой роли | PostgreSQL | TCP 5432 | Запрещено | Нет разрешающего ingress-правила |
| Обычный Pod без роли клиента | `shop-api` | TCP 8080 | Запрещено | Нет разрешающего ingress-правила |
| Pod приложения | CoreDNS | UDP/TCP 53 | Разрешено | Разрешение имён Service |
| Pod приложения | Произвольный внешний адрес | Любой | Запрещено | Не разрешён egress, кроме DNS и PostgreSQL для API/worker |

Политики Kubernetes объединяются по разрешающим правилам: наличие default-deny не отменяет разрешения других NetworkPolicy. Поэтому разрешающие политики ограничены конкретными Pod-ролями, namespace и портами.

<img width="925" height="390" alt="image" src="https://github.com/user-attachments/assets/650ae088-4a56-4f14-b113-4c80692469da" />

## 6. Проверка разрешённых и запрещённых соединений

```bash
make test-network
```

Скрипт создаёт временные BusyBox Pod с разными `network-role`, проверяет TCP-соединения к PostgreSQL и API, а затем удаляет диагностические Pod. Ожидается:

- `api-probe → shop-postgres-rw:5432` — соединение разрешено;
- `worker-probe → shop-postgres-rw:5432` — соединение разрешено;
- `denied-probe → shop-postgres-rw:5432` — соединение блокируется;
- `api-client → shop-api:80` — соединение разрешено;
- `denied-probe → shop-api:80` — соединение блокируется.

<img width="921" height="524" alt="image" src="https://github.com/user-attachments/assets/4ecf7250-890d-40fd-bfec-f1fab8b509ed" />

## 7. Устойчивость Service при удалении Pod

Проверить, что Service продолжает направлять запросы на оставшиеся реплики:

```bash
kubectl get pods -l app=shop-api -o wide
kubectl delete pod "$(kubectl get pods -l app=shop-api -o jsonpath='{.items[0].metadata.name}')"
kubectl rollout status deployment/shop-api --timeout=120s
curl -i http://127.0.0.1:30080/health
kubectl get pods -l app=shop-api -o wide
```

Deployment создаёт замену удалённому Pod. Service сохраняет стабильный адрес и выбирает доступные endpoints.
На коротком промежутке во время завершения/создания Pod отдельный запрос может не пройти; поэтому нужно выполнить несколько запросов после восстановления готовности.

<img width="925" height="755" alt="image" src="https://github.com/user-attachments/assets/65d31150-f6b3-43f0-8710-15bdef8ad7b7" />

<img width="926" height="926" alt="image" src="https://github.com/user-attachments/assets/ecadd204-c96f-40a8-b1ab-bd3c1125821f" />


## 8. Мониторинг и алертинг

Для сбора метрик и визуализации используется стек Prometheus и Grafana, развернутый в предыдущих лабораторных.
В Helm-чарт добавлены `ServiceMonitor` для сбора метрик с API и Cilium, а также `PrometheusRule`.

1. **API request rate и API error rate:** Ровная зеленая линия на уровне ~1 запроса в секунду доказывает, что API стабильно обрабатывает запросы. При этом график ошибок (`API error rate`) находится на нуле — значит, все запросы доходят до базы данных, потерь нет.
2. **TCP connection events:** Показывает сетевую активность на уровне TCP (SYN и RST пакеты). Это подтверждает, что Service корректно перенаправляет трафик.
3. **Dropped traffic:** Желтая линия **`POLICY_DENIED / TCP`** — это результат работы нашего тестового пода-нарушителя (`intruder-probe`), который пытался достучаться до базы данных. Сетевая политика Cilium мгновенно заблокировала этот трафик, и Grafana зафиксировала его как отброшенный. Зеленая линия `UNSUPPORTED_L3_PROTOCOL / ICMPv6` — это служебный IPv6-трафик, который отбрасывается в нашем IPv4-кластере (это норма).

<img width="1748" height="779" alt="image" src="https://github.com/user-attachments/assets/b5ee9cb2-d8ad-48a8-8c2e-77cbce62cc5f" />

1. **Hubble flow verdicts:** Стабильный график `FORWARDED / TCP` (около 45 пакетов в секунду) доказывает, что разрешенный трафик (API -> БД, Prometheus -> API) успешно проходит через сеть.
2. Также видны единичные отбросы `DROPPED / ICMPv6` — это служебный IPv6-трафик, который Cilium блокирует по умолчанию в нашем IPv4-кластере.
3. **CoreDNS request rate и responses:** Показывают, что кластер активно использует DNS для разрешения имен сервисов (запросы UDP/A и ответы NOERROR), при этом есть редкие запросы NXDOMAIN (обращение к несуществующим именам).
4. **CoreDNS request duration p95:** Задержка DNS-запросов в основном держится на уровне 0, с редкими всплесками до 400 мс при первичном обращении.

<img width="1762" height="637" alt="image" src="https://github.com/user-attachments/assets/f9ac3cfb-7286-4f74-8fd9-86ae363eb5e8" />

Для проверки работоспособности алертинга был запущен временный под-нарушитель (`intruder-probe`), который пытался подключиться к базе данных по заблокированному порту.
Это вызвало всплеск метрики `POLICY_DENIED`, и система мониторинга мгновенно отреагировала.

<img width="1740" height="283" alt="image" src="https://github.com/user-attachments/assets/3e0b1516-0390-437f-af9f-198cfc8ab6f4" />

## 9. Визуализация потоков и диагностика в Hubble

Для наглядного отображения работы сетевых политик используется инструмент Hubble UI, который входит в стек Cilium. Он позволяет в реальном времени видеть, какие соединения разрешены, а какие блокируются.

На скриншоте ниже представлен граф потоков в namespace `shop`:

<img width="1423" height="766" alt="image" src="https://github.com/user-attachments/assets/24e26439-a83e-4d7f-b7b6-23acffd22689" />

1. **Граф (сверху):** 
   * Зеленые линии показывают разрешенный трафик: `worker` успешно подключается к `postgres` (порт 5432), а `prometheus` собирает метрики с `api` (порт 8080).
   * **Красная линия** — это заблокированный трафик. Оператор `cloudnative-pg` (из namespace `cnpg-system`) пытается достучаться до `postgres`, но сетевая политика (NetworkPolicy) запрещает это соединение.
2. **Таблица потоков (снизу):** 
   * Колонка `Verdict` показывает результат: `forwarded` (разрешено) для разрешенных потоков и `dropped` (отброшено) для запрещенных.
   * Видно, что `cloudnative-pg` пытается подключиться к порту `8000`, но получает `dropped`. Это доказывает, что политика **default-deny** работает и блокирует доступ из других namespace'ов к базе данных, кроме разрешенных.

Этот же инструмент использовался для поиска «скрытой поломки» (когда мы намеренно нарушили связь API с БД): по красным линиям в Hubble можно мгновенно определить, на каком именно порту и между какими подами блокируется соединение.

