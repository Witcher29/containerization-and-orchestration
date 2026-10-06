## Часть 0 — Сервисы

Сгенерированы два сервиса: api (HTTP) и worker (фоновый обработчик заказов). Реализация — Go, оценка за код не идёт, важно, чтобы оба собирались и работали.

api — HTTP-сервис с тремя эндпоинтами:

GET /health — возвращает ok, но 500, если переменная окружения HEALTH_FAIL=true. Это понадобится в части 2 для симуляции сломанной версии при rolling update.

POST /order — создаёт заказ в Postgres (insert со статусом new).

GET /orders — возвращает список заказов.

worker — фоновый воркер. Раз в 5 секунд забирает из Postgres заказы со статусом new и помечает их как done. Никаких портов не слушает, только работает с БД.

Оба сервиса подключаются к Postgres с ретраями (до 30 попыток по 2 секунды) — это нужно, потому что Postgres в Kubernetes может стартовать позже приложения. Параметры подключения (host, port, user, password, db) — через переменные окружения.

Загрузка в kind. Образы из Docker-демона хоста не видны узлам kind автоматически, поэтому используются команды:

kind load docker-image shop-api:latest --name observability-demo
kind load docker-image shop-worker:latest --name observability-demo

<img width="924" height="293" alt="image" src="https://github.com/user-attachments/assets/d5091206-dffe-47b1-8f59-a27d5269f255" />


## Часть 1 — Политики кластера (Kyverno)

### Выбор механизма

Выбран Kyverno, а не Gatekeeper. Причина простая: Kyverno ставится одним Helm-чартом, пишет правила на YAML или на CEL, и для локального кластера порог вхождения ниже.

Как это работает: Kyverno — admission controller. Когда мы делаем kubectl apply или helm install, Kubernetes сначала отправляет объект не в etcd, а всем зарегистрированным webhook'ам. Kyverno перехватывает запрос, проверяет по своим политикам. Если нарушение — возвращает ошибку, объект не создаётся. Если всё ок — объект уходит дальше.

### Политики

Пять правил
1. require-resources — обязательные limits и requests.

Каждый контейнер обязан иметь resources.limits и resources.requests для cpu и memory. Без limits под может выесть память узла и уронить соседей. Без requests планировщик не знает, сколько ресурсов резервировать, и может перегрузить узел.

2. trusted-registries — только доверенные реестры.

Разрешены образы из gcr.io/distroless/*, docker.io/library/* и локальных shop-*. Запрещает тянуть образы из неизвестных реестров, где может быть малварь или бэкдор. Также защищает от опечаток: если кто-то напишет docker.io/librari/busybox, политика отклонит.

3. require-labels — обязательные лейблы.

Требуются лейблы app, owner, env. Без них невозможно фильтровать ресурсы, привязывать ServiceMonitor, разграничивать доступ и понимать, чей это под и зачем.

4. disallow-privileged — запрет привилегированных контейнеров.

securityContext.privileged: true даёт контейнеру практически полный доступ к ядру хоста

5. disallow-latest-tag — запрет тега :latest.

Тег :latest не привязан к конкретной версии образа. При перезапуске пода можно получить другой код, rollback становится невозможным, а imagePullPolicy: IfNotPresent может привести к непредсказуемому поведению. Также запрещены образы без явного тега (например, просто busybox).

Применили политики:

<img width="928" height="237" alt="image" src="https://github.com/user-attachments/assets/967d42a2-a9cf-485f-a9c9-85668ab01532" />

Проверили работу политик с помощью подов, нарушающих каждую из них:

<img width="930" height="427" alt="image" src="https://github.com/user-attachments/assets/ab94c8c5-fcc3-4c73-a30e-45ef39989ede" />

## Часть 2

### Helm-чарт shop

Чарт charts/shop/ разворачивает три сервиса: api, worker, postgres.
В values.yaml заданы:
- api.replicaCount: 3 — три реплики API;
- worker.replicaCount: 2 — две реплики воркера;
- образы shop-api:v1 и shop-worker:v1 (без префикса реестра — политика trusted-registries разрешает образы, начинающиеся с shop-);
- ostgres.image.repository: docker.io/library/postgres, тег 16-alpine (с полным префиксом — политика разрешает docker.io/library/*);
- resources.limits и resources.requests для всех контейнеров (требование require-resources);
- общие лейблы owner: team-shop, env: dev (вместе с app на каждом поде — требование require-labels).

Все три образа — с явными тегами (v1, 16-alpine), без privileged, из доверенных источников. Чарт проходит все пять политик Kyverno.

При первом запуске shop-api и shop-worker имели несколько рестартов: приложение на старте блокируется на подключении к Postgres (до 60 секунд с ретраями), а liveness probe с initialDelaySeconds: 10 начинает бить /health раньше, чем HTTP-сервер поднимается. После того как Postgres инициализируется, поды становятся стабильными. Для устранения рестартов initialDelaySeconds liveness можно увеличить до 60.

Проверка работы:

<img width="926" height="209" alt="image" src="https://github.com/user-attachments/assets/476135b9-f70a-4379-8476-dfb0bfc788c1" />

### Самовосстановление подов

Удаление пода вручную приводит к его пересозданию. Deployment следит за количеством реплик и сам восстанавливает недостающие поды:

<img width="922" height="228" alt="image" src="https://github.com/user-attachments/assets/f8a85b70-82eb-4194-b307-5f0f959c121d" />

### Rolling update

Изменение тега образа в values.yaml (или через --set api.image.tag=v2) и helm upgrade запускает rolling update.
Kubernetes обновляет поды по одному: создаёт новый, ждёт Ready, только после этого удаляет один старый. Всё время обновления хотя бы 2 из 3 реплик api остаются доступными.:

<img width="924" height="584" alt="image" src="https://github.com/user-attachments/assets/5681f50e-5b8d-4086-b69f-d3151adc1d7d" />

Поэтому нет потери соеденения во время обновления:

<img width="928" height="947" alt="image" src="https://github.com/user-attachments/assets/36bb894b-2a8a-4bd1-a568-080dba8c5abd" />

История ревизий (4, а не 2, потому чо вручную пробовали менять ревизии перед обновлением):

<img width="927" height="136" alt="image" src="https://github.com/user-attachments/assets/a64ff88d-8f6f-4d1b-ab5e-2ba35b2d1805" />

Смена ревизий:

<img width="924" height="584" alt="image" src="https://github.com/user-attachments/assets/47af9205-1354-48fc-9f3d-8d7c51c436ce" />

### Сломанная версия и откат

Развёрнута намеренно неисправная версия, после чего выполнен откат к исправной и проверена работа сервиса:

<img width="927" height="298" alt="image" src="https://github.com/user-attachments/assets/fff420bc-2ea5-42ef-8296-f1326042ef58" />

При HEALTH_FAIL=true эндпоинт /health возвращает 500. Что происходит:
- Readiness probe новых подов не проходит → новый под не становится Ready.
- Liveness probe получает 500 → Kubernetes перезапускает контейнер → под уходит в CrashLoopBackOff.
- Rolling update не может удалить ни один старый под: новый должен стать Ready, а он не становится. Все три старых пода остаются 1/1 Running и продолжают обслуживать трафик.
- Service направляет запросы только на Ready-endpoints — на сломанные поды трафик не идёт.
- Через 5 минут helm upgrade падает с context deadline exceeded.

Во время helm upgrade создался 4-й под, который постоянно рестартается, но первые три не удалились:

<img width="804" height="208" alt="image" src="https://github.com/user-attachments/assets/25e4a44f-7479-49b9-808f-1bdce9704471" />

## Часть 3

В ходе проверки был удалён Pod shop-postgres-1. CloudNativePG обнаружил, что фактическое количество экземпляров стало меньше заданного в spec.instances: 1, и восстановил PostgreSQL Pod. После завершения восстановления кластер вернулся в состояние Ready.

<img width="974" height="573" alt="image" src="https://github.com/user-attachments/assets/804c1e25-5be7-4a91-885a-3df670520378" />

### Spec и status объекта PostgreSQL Cluster
CloudNativePG использует Kubernetes Custom Resource Cluster для описания PostgreSQL-кластера.
Поле spec описывает желаемое состояние кластера. В нашем случае оно задаёт один экземпляр PostgreSQL (instances: 1), параметры ресурсов CPU и памяти, размер persistent storage (1Gi), параметры первоначальной инициализации базы shop и PostgreSQL image.

<img width="499" height="123" alt="image" src="https://github.com/user-attachments/assets/fe4e0338-ac31-4af6-a69b-a86c03e8448d" />
<img width="752" height="248" alt="image" src="https://github.com/user-attachments/assets/2f8113e9-17a9-4cec-a092-79b8e8e45bb9" />

Поле status описывает фактическое состояние, которое наблюдает CloudNativePG Operator. В нём содержится информация о текущем primary (shop-postgres-1), количестве экземпляров, PVC, созданных сервисах (shop-postgres-rw, shop-postgres-rw/read services), состоянии сертификатов и текущей фазе кластера.

<img width="974" height="247" alt="image" src="https://github.com/user-attachments/assets/cdaa9a32-f738-4f74-a465-206c2650ad40" />

Таким образом:
  -	spec — что пользователь хочет получить;
  -	status — что оператор фактически наблюдает;
  -	CloudNativePG постоянно сравнивает фактическое состояние с желаемым и выполняет reconciliation.
    
### Отличие оператора от controller-manager
Оператор — это специализированный контроллер Kubernetes, который управляет конкретным типом ресурсов и реализует дополнительную предметную логику.
В данном случае оператор CloudNativePG Operator имеет следующие преимущества над контроллером:
  -	Знает специфику PostgreSQL — CloudNativePG Operator понимает, как правильно запускать, настраивать и восстанавливать PostgreSQL, а обычный controller-manager работает с общими объектами Kubernetes.
  -	Автоматически управляет базой — оператор может сам создавать PostgreSQL Pod, PVC, Services, выполнять recovery и поддерживать нужное количество экземпляров.
  -	Умеет выполнять специальные операции — например, управлять primary/replica, репликацией, backup и failover PostgreSQL. Обычный controller-manager такой логики для базы данных не имеет.

## Часть 4

### Фиксация текущего состояния
Перед проведением эксперимента зафиксировали текущее состояние Kubernetes-кластера и убедились, что все основные компоненты работают штатно. API, worker и PostgreSQL были запущены, а Kubernetes API Server был доступен для выполнения команд управления кластером. Здесь можно увидеть ip-адреса сервисов, к которым будем обращаться напрямую после остановки control plane.

<img width="974" height="222" alt="image" src="https://github.com/user-attachments/assets/94e57d5f-fa2d-4ae1-84eb-367e6c449e6c" />

### Остановка и восстановление control plane
Остановили контейнер control plane, в результате чего Kubernetes API Server стал недоступен, а команды управления кластером перестали выполняться. После этого сразу запустили control plane обратно и повторно выполнили Kubernetes-команды. Увидели, что после восстановления control plane команды снова проходят успешно, а кластер возвращается в рабочее состояние.

<img width="974" height="118" alt="image" src="https://github.com/user-attachments/assets/3ca3fb68-4426-4235-bcb7-9c4de8ce5774" />
<img width="974" height="166" alt="image" src="https://github.com/user-attachments/assets/23d08663-dd9b-4527-be8b-cd6ee3692f6f" />

### Работа API во время остановки control plane
Во время остановки control plane проверили непосредственно работу api-сервиса, отправив HTTP-запросы к его endpoint. Несмотря на недоступность Kubernetes API Server, уже запущенный API-сервис продолжал отвечать на запросы. Это показывает, что отказ control plane не приводит к немедленной остановке уже работающих workload'ов в кластере.

<img width="974" height="138" alt="image" src="https://github.com/user-attachments/assets/493d32c4-b50f-488d-a0be-81aabfa00de8" />


