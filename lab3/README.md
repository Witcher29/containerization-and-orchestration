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

