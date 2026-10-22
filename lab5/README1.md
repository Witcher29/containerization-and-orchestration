# Лабораторная работа 5 — Kubernetes Networking, Calico и NetworkPolicy

> **Статус отчёта:** конфигурация и команды подготовлены под существующий проект `shop` из лабораторных 2–4. Сетевые проверки, графики и скриншоты необходимо получить на своём компьютере после успешного запуска кластера. В местах, отмеченных `ЗАПОЛНИТЬ ПО ФАКТУ`, не нужно оставлять неподтверждённые результаты.

## 1. Цель работы

Развернуть отдельный многонодовый кластер Kubernetes на `kind` с CNI Calico, проверить связь между Pod и Service, выставить API наружу через `NodePort`, ограничить сетевые соединения с помощью `NetworkPolicy`, проверить разрешённые и запрещённые потоки и продемонстрировать диагностику искусственно внесённой сетевой ошибки. Для метрик используется существующий стек Prometheus/Grafana из предыдущих лабораторных.

## 2. Архитектура решения

- **Cluster:** `lab5-calico` — отдельный кластер, чтобы не менять `observability-demo` и `lab4-scheduling`.
- **CNI:** Calico `v3.28.0`, совместимый с используемым образом Kubernetes `v1.29.2`.
- **API:** `shop-api`, порт контейнера `8080`, Service `NodePort` `30080`.
- **Worker:** `shop-worker`, инициирует соединения к PostgreSQL, входящие соединения ему не нужны.
- **Database:** CloudNativePG, Service записи `shop-postgres-rw`, порт `5432`.
- **Observability:** Prometheus/Grafana из чарта предыдущей лабораторной; `ServiceMonitor` собирает `/metrics` с API.

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
```

## 3. Создание кластера с Calico

В `cluster/kind-calico.yaml` отключён стандартный CNI `kindnet`, потому что CNI-интерфейсы и IPAM должен настраивать Calico. Для Pod-сети выбран CIDR `192.168.0.0/16`; он совпадает с пулом Calico в манифесте установки. Host-порт `30080` проброшен в control-plane контейнер kind, чтобы NodePort был доступен с хоста.

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

Если `kind export logs` не срабатывает, проверьте `docker logs lab5-calico-control-plane` (если контейнер остался). Скрипт создания использует `--retain`, чтобы сохранить узлы для диагностики при неудаче. Не удаляйте остальные кластеры и не запускайте `docker system prune`.

### Скриншот 1 — кластер и Calico

Сохранить как `screenshots/01-calico-nodes.png`.

![Место для скриншота 1 — kubectl get nodes и Calico pods](screenshots/01-calico-nodes.png)

> **Вставить:** вывод `kubectl get nodes -o wide` и `kubectl get pods -n kube-system | grep calico` с состояниями `Ready`/`Running`.

## 4. Развёртывание приложения

```bash
make deploy
kubectl get pods,svc -o wide
kubectl get networkpolicy
```

Скрипт устанавливает CloudNativePG operator, стек наблюдаемости, собирает локальные образы API/worker/batch, загружает их в `lab5-calico` и устанавливает чарт `shop`. Существующие лабораторные директории не изменяются.

Проверка API с хоста:

```bash
curl -i http://127.0.0.1:30080/health
```

Ожидается `HTTP/1.1 200 OK` и тело `ok`. Создание заказа проверяется запросом:

```bash
curl -i -X POST http://127.0.0.1:30080/order
```

Ожидается успешный ответ JSON с `id` и `status`. Если база ещё инициализируется, дождитесь готовности `shop-postgres` и повторите запрос.

### Скриншот 2 — Service и NodePort

Сохранить как `screenshots/02-nodeport-api.png`.

![Место для скриншота 2 — Service NodePort и curl](screenshots/02-nodeport-api.png)

> **Вставить:** `kubectl get svc shop-api` и успешный `curl http://127.0.0.1:30080/health`.

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

### Скриншот 3 — NetworkPolicy

Сохранить как `screenshots/03-network-policies.png`.

![Место для скриншота 3 — список NetworkPolicy](screenshots/03-network-policies.png)

> **Вставить:** `kubectl get networkpolicy` и при необходимости `kubectl describe networkpolicy shop-postgres-ingress`.

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

**ЗАПОЛНИТЬ ПО ФАКТУ:** результат запуска `make test-network`: ____________________.

### Скриншот 4 — проверка матрицы доступа

Сохранить как `screenshots/04-connectivity-tests.png`.

![Место для скриншота 4 — успешные и заблокированные подключения](screenshots/04-connectivity-tests.png)

> **Вставить:** вывод `make test-network`, где видны разрешённые подключения и ожидаемые ошибки для запрещённых.

## 7. Устойчивость Service при удалении Pod

Проверить, что Service продолжает направлять запросы на оставшиеся реплики:

```bash
kubectl get pods -l app=shop-api -o wide
kubectl delete pod "$(kubectl get pods -l app=shop-api -o jsonpath='{.items[0].metadata.name}')"
kubectl rollout status deployment/shop-api --timeout=120s
curl -i http://127.0.0.1:30080/health
kubectl get pods -l app=shop-api -o wide
```

Deployment создаёт замену удалённому Pod. Service сохраняет стабильный адрес и выбирает доступные endpoints. На коротком промежутке во время завершения/создания Pod отдельный запрос может не пройти; поэтому нужно выполнить несколько запросов после восстановления готовности.

### Скриншот 5 — восстановление после удаления Pod

Сохранить как `screenshots/05-service-resilience.png`.

![Место для скриншота 5 — удаление и восстановление API pod](screenshots/05-service-resilience.png)

> **Вставить:** до/после `kubectl get pods -l app=shop-api -o wide`, успешный rollout и curl.

## 8. Нагрузочное тестирование и мониторинг

Запустить тест из каталога `lab5`:

```bash
make load
```

Если `k6` не установлен, используйте Docker-команду, которую выводит скрипт `scripts/load-test.sh`. По умолчанию сценарий обращается к `http://127.0.0.1:30080`.

Prometheus/Grafana можно открыть так:

```bash
make port-forward-prometheus
# В другом терминале:
make port-forward-grafana
```

- Prometheus: `http://127.0.0.1:9090`
- Grafana: `http://127.0.0.1:3000` (логин/пароль из `observability/values.yaml`; в текущем конфиге `admin/admin`).

В Helm-чарт добавлен `ServiceMonitor` для `/metrics` и три правила:

1. **`ShopApiTargetDown`** — Prometheus не может опросить API; помогает найти проблему ServiceMonitor, endpoints или ingress NetworkPolicy.
2. **`ShopApiHighErrorRate`** — появляются HTTP 5xx; помогает обнаружить сбой API→PostgreSQL или другую серверную ошибку.
3. **`ShopApiHighLatency`** — p95 превышает 300 мс; указывает на деградацию задержки под нагрузкой.

**ЗАПОЛНИТЬ ПО ФАКТУ:** длительность теста, количество запросов, p95, процент ошибок: ____________________.

### Скриншот 6 — метрики и нагрузочный тест

Сохранить как `screenshots/06-k6-summary.png`.

![Место для скриншота 6 — summary k6](screenshots/06-k6-summary.png)

### Скриншот 7 — Prometheus / Grafana

Сохранить как `screenshots/07-prometheus-grafana.png`.

![Место для скриншота 7 — метрики API или панель Grafana](screenshots/07-prometheus-grafana.png)

### Скриншот 8 — правила алертов

Сохранить как `screenshots/08-alert-rules.png`.

![Место для скриншота 8 — три правила в Prometheus](screenshots/08-alert-rules.png)

## 9. Диагностика искусственной сетевой ошибки

Сценарий удаляет разрешающее правило egress для API. Это намеренно ломает соединение `shop-api → PostgreSQL`, при этом Pod и Service API остаются запущены. Внешне это проявляется как HTTP 500 при создании заказа; причина видна в логах API, а метрика `http_requests_total` растёт по коду `5..`.

1. Перед экспериментом убедиться, что API работает: `curl -i -X POST http://127.0.0.1:30080/order`.
2. Запустить `make fault`.
3. В течение 30–60 секунд повторить POST-запросы.
4. Посмотреть `kubectl logs deploy/shop-api --since=2m` и проверить `ShopApiHighErrorRate` в Prometheus.
5. Немедленно восстановить политику: `make restore`.
6. Убедиться, что POST снова проходит и количество ошибок перестало расти.

Не оставляйте политику удалённой после эксперимента: default-deny продолжит блокировать egress API, пока Helm не восстановит `shop-api-egress`.

### Скриншот 9 — обнаружение и исправление ошибки

Сохранить как `screenshots/09-network-fault.png`.

![Место для скриншота 9 — HTTP 500, лог ошибки и firing alert](screenshots/09-network-fault.png)

> **Вставить:** запрос с HTTP 500, лог ошибки подключения к PostgreSQL и alert в состоянии `firing`. Отдельно сохранить подтверждение восстановления после `make restore`.

## 10. Итоговые выводы

В работе создаётся отдельный кластер `kind` с Calico, что позволяет применять сетевые политики. `NodePort` предоставляет API с хоста, а `Service` сохраняет стабильную точку доступа при замене Pod. `NetworkPolicy` реализует default-deny и разрешает только необходимые потоки: API/worker к PostgreSQL, DNS и сбор метрик Prometheus. Проверка матрицы доступа демонстрирует не только успешные соединения, но и блокировку неразрешённых потоков. Внесение ошибки через удаление egress-правила показывает, как сопоставлять HTTP 5xx, логи приложения, сетевые политики и Prometheus alerts.

**Финальный статус выполнения:**

- [ ] Кластер `lab5-calico` создан, все узлы `Ready`.
- [ ] Calico работает на всех узлах.
- [ ] API доступен на `http://127.0.0.1:30080`.
- [ ] Матрица NetworkPolicy проверена, отрицательные тесты действительно блокируются.
- [ ] Проверено восстановление API Pod после удаления.
- [ ] Нагрузочный тест выполнен; реальные p95 и error rate внесены выше.
- [ ] Проверены три алерта; хотя бы один реально наблюдался в `firing` при контролируемой неисправности.
- [ ] Все скриншоты сохранены в `screenshots/`.

## 11. Очистка

Удалить только кластер этой лабораторной:

```bash
make cluster-down
```

Команда не затрагивает `observability-demo` и `lab4-scheduling`.
