## Часть 0 — Сервис batch

Сгенерирован сервис batch, который загружает память в бесконечном цикле.

Соберем контейнер:

<img width="1015" height="508" alt="Screenshot from 2026-10-06 20-55-31" src="https://github.com/user-attachments/assets/1687502b-59ed-44a9-bc41-b87190582ff7" />

Проверим, что контейнер заполняет память с помощью утилиты `docker stats`:

<img width="822" height="128" alt="Screenshot from 2026-10-06 20-54-27" src="https://github.com/user-attachments/assets/deb18140-d336-4249-b174-73daf49c3c47" />


## Часть 1 — Выбери механизм разнесения

### Выбор механизма 

Для распределения реплик api между узлами Kubernetes добавлен параметр `api.scheduling.strategy` в `values.yaml`. Он позволяет выбрать один из двух механизмов - `podAntiAffinity` и `topologySpreadConstraints`.

`podAntiAffinity` задает правило, запрещающее размещать выбранные под в одной домене топологии. В нашем случае используется `kubernetes.io/hostname`, поэтому реплики api не должны размещаться на одной ноде.

`topologySpreadConstraints` задает более точное требование к распределению подов. В конфигурации используется `maxSkew: 1`, поэтому количество реплик api на разных нодах должно отличаться не более чем на единицу. Параметр `whenUnsatisfiable: DoNotSchedule` запрещает планировщику размещать под, если требование распределения выполнить невозможно.

Выбран `topologySpreadConstraints`, так как он позволяет не только запретить совместное размещение реплик, но и явно задать степень равномерности их распределения. При двух нодах и трех репликах api ожидается распределение `2+1`, а при наличии трёх нод - `1+1+1`.

Проверим, что выбран верный механизм:
<img width="1011" height="474" alt="Screenshot from 2026-10-06 21-15-44" src="https://github.com/user-attachments/assets/e67b2431-10b6-4f7d-b8d9-130c444cd132" />

## Часть 2 - Тесная площадка

Кластер поднят строго на 1 control-plane и 2 worker-ноды через kind (файл cluster/kind-config.yaml, cluster name lab4-scheduling). Намеренно мало ресурсов — 4 CPU / 8 GB на всю VM, чтобы нехватку поймать без шаманства.

<img width="407" height="56" alt="image" src="https://github.com/user-attachments/assets/22cc798b-9b3a-48b8-b408-6889232a5217" />

Добавил batch в чарт shop новым шаблоном templates/batch-deployment.yaml, параметры — в values.yaml в секции batch. Лимиты есть, как и требуют гвардрейлы Лабы 3: requests: 100m/128Mi, limits: 500m/512Mi. Образ shop-batch:v1 собирается из service/batch/Dockerfile и грузится в kind через новую цель в Makefile.

Сам batch — Go-приложение, которое выходит на плато по памяти (BATCH_MEMORY_TARGET_MIB, по умолчанию 200 MiB), держит её и параллельно жжёт CPU. Поведение рулится через env, так что для Частей 6 и 7 не придётся пересобирать образ — только values.yaml и helm upgrade.

<img width="668" height="113" alt="image" src="https://github.com/user-attachments/assets/32072500-cf3a-416a-aed9-78858e741db8" />

Убедился, что batch действительно живёт на плато и не крашится:

<img width="493" height="37" alt="image" src="https://github.com/user-attachments/assets/e079d985-11da-45b8-a382-2abb2e3c9222" />
<img width="692" height="26" alt="image" src="https://github.com/user-attachments/assets/7dc27503-44ef-4a33-9da6-08ea86df4529" />


batch развёрнут как часть чарта shop, с разумными requests/limits и Burstable-классом, стабильно работает и не мешает критическому пути. Кластер намеренно тесный — под Части 6 и 7 будет где давить.

## Часть 3 - Реальные числа вместо угадывания

Дали k6 держать 10 VUs на POST /order в течение 15 минут. Использовался скрипт load/order.js
После завершения k6 вывел итоговую сводку.

- http_req_duration p(95) = 5.61 ms — порог <300 ms пройден с большим запасом
- http_req_failed = 0.00% — порог <0.01 пройден
- 17 460 успешных проверок из 17 460, 8 730 HTTP-запросов
- avg = 3.9 ms, med = 3.51 ms, max = 276 ms
- 
<img width="974" height="650" alt="image" src="https://github.com/user-attachments/assets/0dbf5e44-d340-4ad3-83f1-cafcd770337b" />

Заодно убедились, что заказы реально доходят: в логах воркера непрерывно order N marked as done. Это подтверждает, что нагрузка на api дошла и до postgres, и до worker'а.

<img width="479" height="131" alt="image" src="https://github.com/user-attachments/assets/82cb78fe-ce39-489b-bfed-6811db8f7bbf" />

### Результат рекомендаций krr:
Для получения таблицы с рекомендациями была выполнена команда:
`krr simple -n default --prometheus-url http://localhost:9090 --history_duration 2 --points_required 20`

<img width="974" height="439" alt="image" src="https://github.com/user-attachments/assets/bb6c01ac-4fa0-4237-98e8-706863cb4db8" />

<img width="427" height="97" alt="image" src="https://github.com/user-attachments/assets/569a3003-bdfa-4c72-88d3-f51331cdcd93" />

### Определение показателей для postgres
Prometheus не понимает CNPG как workload, но cAdvisor собирает метрики по поду <cluster>-1 — их достаточно.

Для определения cpu использовался PromQL запрос:
`quantile_over_time(0.95, rate(container_cpu_usage_seconds_total{namespace="default",pod=~"shop-postgres.*",container!=""}[5m])[1h:1m])`

Резульат = 20m

<img width="974" height="417" alt="image" src="https://github.com/user-attachments/assets/442a8773-bc42-459c-bf55-c244ab0daf13" />

А для памяти:

`quantile_over_time(0.95, container_memory_working_set_bytes{namespace="default",pod=~"shop-postgres.*",container!=""}[1h:1m]) / 1024 / 1024`

Результат = 96 MB

<img width="974" height="417" alt="image" src="https://github.com/user-attachments/assets/595fcd35-76a9-432c-8ce6-fac312dbf3b0" />

Ниже приведена таблица с расхождениями в потреблении ресурсов. Красным выделил сервис batch по причине того, что именно его ресурсы KRR потребовал увеличить.  А для остальных надо наоборот снизить выделение ресурсов:

<img width="440" height="322" alt="image" src="https://github.com/user-attachments/assets/fd352573-e21b-4c86-b187-2111eb070bd2" />

Главный вывод: мы переоценили requests api/worker/postgres примерно в 2–5 раз. Классический «ставим на глаз с запасом» → krr показывает реальный пол. Для batch — наоборот, krr увидел то, что мы сами недооценили: burn-цикл жрёт CPU.

## Часть 4 - Разнесение критического сервиса

Механизм разнесения — `topologySpreadConstraints`.
На скриншоте видно, что shop-api-...-nm69x крутится на lab4-scheduling-worker, а shop-api-...-snvp2 — на lab4-scheduling-worker2. Чистое 1+1, никаких «двух на одной ноде». Обе в Running, обе Ready 1/1.

<img width="644" height="51" alt="image" src="https://github.com/user-attachments/assets/eacf778a-e8f7-4816-a1b2-11bfc27b8f55" />

В манифесте видно ровно то, что мы задавали в values.yaml:

`maxSkew`: `1`

`topologyKey`: `"kubernetes.io/hostname"`

`whenUnsatisfiable`: `"DoNotSchedule"`

`labelSelector.matchLabels.app`: `"shop-api"`

Это важно: механизм описан декларативно в чарте, а не прилетел разово через kubectl edit. Значит, при следующем helm upgrade поведение сохранится, и его нельзя случайно «потерять».

<img width="224" height="139" alt="image" src="https://github.com/user-attachments/assets/57729245-b8f2-4eb1-8647-aa7a78c2d84f" />

### Распределение подов в namespace default

В данном случае видно не только про api:

<img width="400" height="109" alt="image" src="https://github.com/user-attachments/assets/94942e37-9884-46ec-9934-e7b4f47df124" />

- shop-api — по одному поду на каждой ноде.
- shop-worker — тоже 1+1, по одному на worker и worker2.
- shop-batch — один под на worker2 (у него всего 1 реплика, spread его ни к чему не обязывает).
- shop-postgres-1 — на worker2.
- shop-postgres-init — Succeeded, это Job, ему быть одновременно нигде не нужно.

### Отказоусточйивость

Если api держится одной нодой — падение этой ноды кладёт весь сервис. У нас api разнесён, и это даёт сразу три эффекта.

- Падение ноды не вырубает сервис.
Если worker целиком уйдёт в даун (kubelet креш, сеть, drain), Service shop-api автоматически выкинет под с мёртвой ноды из endpoints. Вся нагрузка пойдёт на вторую реплику на worker2. k6 под нагрузкой в такой момент может поймать пару секунд ошибок на переключении, но не длительный отказ.

- Обновления и drain не блокируют трафик.
При kubectl drain worker под перестаёт принимать трафик, но не уходит мгновенно — ему дают terminationGracePeriodSeconds. Вторая реплика в это время спокойно обслуживает. PDB из Части 5 дополнительно защитит от ситуации, когда drain попытается снести обе реплики одновременно.

## Часть 5 - Приоритеты и гарантии

Добавили в Helm-чарт shop шаблоны priorityclass.yaml и pdb.yaml. В values.yaml настроили классы для критического пути и фоновой нагрузки. Для api и postgres выставили requests = limits (QoS Guaranteed) и высокий приоритет shop-critical. Для batch оставили requests < limits (QoS Burstable), лимиты на месте (гвардрейлы соблюдены), но назначили низкий приоритет shop-batch.
### Проверка QoS-классов
Здесь мы наглядно видим, как Kubernetes распределил поды по классам обслуживания. Критический путь — api и postgres — получил статус Guaranteed (потому что requests равны limits). А фоновая нагрузка batch и worker осталась в Burstable, так как их requests заметно ниже лимитов. Именно такое разделение позволит системе под давлением по памяти вытеснить batch первым, сохранив жизнь критическим серви

<img width="974" height="138" alt="image" src="https://github.com/user-attachments/assets/1342a5f1-7e35-4c98-9081-8afe422427e1" />

### Проверка назначенных PriorityClass
На этом скрине видно, что приоритеты расставлены корректно: у api и postgres стоит высокий класс shop-critical, а у batch — низкий shop-batch. У worker и init-джобы приоритет не задан (там <none>), и это нормально, они не участвуют в сценариях вытеснения. Благодаря этой настройке планировщик при нехватке места сможет «подвинуть» батч, чтобы посадить на ноду важный под апи.

<img width="974" height="132" alt="image" src="https://github.com/user-attachments/assets/9a6800b1-97d4-45a9-95ac-93798cd0d752" />

### Проверка PDB
Тут зафиксирована работа PDB. Для api мы видим наш ручной shop-api-pdb с одной разрешенной диструпцией (потому что реплик две, и одну можно безопасно выключить при обновлении ноды). А для postgres оператор CloudNativePG автоматически создал shop-postgres-primary с ALLOWED DISRUPTIONS: 0 — это железобетонная защита единственной базы данных от случайного вытеснения во время kubectl drain.

<img width="974" height="111" alt="image" src="https://github.com/user-attachments/assets/e44c7d06-3121-4e2c-83d0-11e3cb2dee56" />

### Инцидент с базой данных
Изначально мы взяли для postgres цифры из Части 3 (krr рекомендовал 20m CPU / 128Mi RAM). Но когда мы сделали их Guaranteed (requests=limits), база не смогла запуститься. CNPG ушёл в Cluster Is Not Ready, а за ним в CrashLoopBackOff упали api и worker, так как им не к кому было подключаться.

<img width="974" height="386" alt="image" src="https://github.com/user-attachments/assets/f5703711-f93e-427e-b20d-ba6e73d8a75f" />

Причина: krr собирает метрики уже работающего пода под лёгкой нагрузкой. Но при старте Postgres запускает initdb, поднимает WAL и shared_buffers, потребляя ресурсы скачкообразно. 20m CPU (2% ядра) и 128Mi оказалось слишком мало, контейнер душило планировщиком. Решение: мы скорректировали requests/limits для postgres до 200m / 512Mi, оставив их равными (Guaranteed). База ожила, api и worker поднялись.

## Часть 6 - Создание нехватки, ловля preemption

Мы подняли batch.replicas до 25. Часть подов ушла в Pending. В событии FailedScheduling планировщик четко указал причину: 0/3 nodes are available: ... 2 Insufficient cpu. Это доказывает, что планировщик оценивает доступность ресурсов строго по параметру requests, а не по фактическому потреблению или лимитам. Так как сумма запрошенных CPU превысила доступную емкость worker-нод, новые поды не были размещены.
Более того, в событии видно preemption: ... 2 No preemption victims found. Это означает, что планировщик попытался освободить место через вытеснение, но не нашел жертв, так как все поды на нодах имели такой же низкий приоритет (shop-batch), как и входящий под. Убийство равного по приоритету не допускается.

<img width="974" height="195" alt="image" src="https://github.com/user-attachments/assets/2633a82a-5226-4a0c-ae7f-a6075107e965" />
<img width="974" height="86" alt="image" src="https://github.com/user-attachments/assets/81221702-a0c3-4e1a-af23-623f55a511cd" />

### Preemption (Вытеснение)
После того как кластер был забит низкоприоритетными подами batch (до состояния Pending), мы увеличили количество реплик критического api до 10. Все 10 подов запустились.

<img width="889" height="424" alt="image" src="https://github.com/user-attachments/assets/2ad71904-5a03-4513-8bc5-8a51de0f917d" />

Так как у api PriorityClass: shop-critical (100000), а у batch — shop-batch (-10), планировщик не стал оставлять новые поды api в Pending. В логах событий (kubectl get events) видно: preemption: found a potential placement for pod on node lab4-scheduling-worker, preempting 1 victims, после чего следует Normal Preempted ... Preempted by pod shop-api. Это доказывает, что планировщик выбрал под batch на ноде, вытеснил (убил) его, освободив необходимые requests, и запустил на его месте высокоприоритетный под api.

<img width="974" height="60" alt="image" src="https://github.com/user-attachments/assets/54944f77-d289-451e-9daf-332e352faaae" />

### Проблема с вытеснением batch подов
Столкнулся с проблемой, что жесткий topologySpreadConstraints с DoNotSchedule блокировал Preemption, так как планировщик не мог найти идеально сбалансированное место даже после убийства жертвы, поэтому изначально не получалось запустить 10 api подов, в running все равно было 2, а все остальные в pending. Было принято решение : смена на ScheduleAnyway позволила ему освободить место, пожертвовав идеальным балансом ради запуска критического сервиса — это показывает приоритет доступности над строгой топологией в момент нехватки).


## Часть 7 - Давление по памяти, порядок жертв

Это был самый сложный пункт в лабораторной
Для проверки работы Eviction мы попытались создать нехватку памяти на ноде, подняв потребление batch. Мы увеличили limits.memory до 2Gi (чтобы избежать падения по собственному лимиту) и задали целевую память (memoryTargetMiB) в 1500–1800MiB.
С какими проблемами мы столкнулись:
OOMKilled вместо Eviction: В первой попытке контейнеры batch не успевали съесть память ноды. Из-за накладных расходов Go-рантайма они превышали свой лимит, и ядро Linux (cgroup OOM Killer) убивало их со статусом OOMKilled. Это локальная проблема контейнера, нарушившего свой лимит.

<img width="974" height="368" alt="image" src="https://github.com/user-attachments/assets/a865309d-83ae-4008-9f71-57dfe4a83881" />

### Особенность kind
Во второй попытке выяснилось, что kind по умолчанию отключает автоматическое вытеснение по нехватке памяти (hard eviction), устанавливая порог memory.available в 1 байт. Даже при явном указании порога через kubeletExtraArgs (1000Mi), глобальный OOM Killer виртуальной машины срабатывал быстрее, чем kubelet успевал корректно вытеснить поды.
В итоге так и не получилось добиться eviction, причем после экспериментов на следующей день ноутбук стал шуметь как вертолёт, поэтому пришлось выбрать легкий путь.
### Как мы решили (Демонстрация):
Чтобы не ронять всю ВМ и показать штатный механизм Kubernetes, мы инициировали вытеснение через Eviction API (POST /api/v1/namespaces/default/pods/<name>/eviction). Этот API используется самим kubectl drain при обслуживании нод.
В результате под batch получил статус Evicted с причиной The node was low on resource: memory, а ReplicaSet мгновенно создал ему замену. При этом критический путь (api, postgres — QoS Guaranteed) продолжил работать без перезапусков.

<img width="974" height="338" alt="image" src="https://github.com/user-attachments/assets/aee9ec33-268a-424a-b234-ae4c3bd9b881" />

### Eviction vs OOMKilled — два разных диагноза:
OOMKilled (Exit Code 137) — это убийство контейнера ядром Linux за превышение собственного limits.memory. Контейнер «жадничает». Как раз OOMKilled постоянно отображался в status у batch’ей.
Eviction (Reason: Evicted) — это решение Kubelet на уровне ноды, когда память закончилась вообще. Kubelet действует по QoS: сначала убивает BestEffort, затем Burstable (наш batch с низким приоритетом), и в последнюю очередь Guaranteed (api, postgres).


### Часть 8 - SLA под нагрузкой

Сформулированный SLA: «95% заказов (p95) должны обрабатываться быстрее 300 мс, а доля ошибок (5xx) не должна превышать 1% даже при наличии фоновой нагрузки».
### Результаты:
Мы запустили нагрузочное тестирование через k6 (20 виртуальных пользователей). Одновременно с этим в кластере активно работал фоновый сервис batch, который потреблял ресурсы CPU.
По итогам k6 видно, что p95 задержки составил всего 16.49 мс (при пороге 300 мс). Доля ошибок — 0.00%. Критический путь успешно выдержал нагрузку.

<img width="974" height="666" alt="image" src="https://github.com/user-attachments/assets/bd725ecc-9bf4-45ed-b730-18f05cbf575b" />

### Деградация batch
На графике потребления CPU из Prometheus видно четкое разделение. Потребление api (зеленая линия) остается минимальным, так как он защищен Guaranteed-классом и имеет высокий приоритет. В то же время batch (коричневая линия) упирается в свой limits.cpu: 700m, и его потребление держится на отметке 0.7 ядра. Это доказывает, что под нагрузкой деградирует именно фоновая нагрузка, ограниченная лимитами, а критический путь продолжает работать без задержек.

<img width="649" height="297" alt="image" src="https://github.com/user-attachments/assets/2d73faab-42fb-4cee-b718-2be9dba7a148" />

### График p95 задержки API (histogram_quantile)
На этом графике показана 95-я перцентиль задержки (p95) для всего сервиса api-service (все реплики агрегированы в одну зелёную линию).
Видно, что в обычном режиме задержка держится около 4-5 миллисекунд. В момент пиковой нагрузки от k6 (около 17:09) линия совершает скачок до 25 миллисекунд, после чего снова падает.

<img width="974" height="449" alt="image" src="https://github.com/user-attachments/assets/64dad5cf-5cc6-4aba-9e77-e283f3876036" />


### CPU Throttling
Можно увидеть всплески throttling — это нормально. У нас batch и другие сервисы жестко ограничены лимитами (гвардрейлы). Но благодаря этому ограничению, даже когда фоновые процессы упираются в свой потолок CPU и тормозятся системой (как коричневая линия на предыдущем графике), задержка критического api (График 1) остается минимальной.

<img width="659" height="291" alt="image" src="https://github.com/user-attachments/assets/e1a7d342-fe17-470d-9cfe-1645dc28446a" />

## Часть 9 - Мониторинг
В Helm-чарт shop были добавлены шаблоны ServiceMonitor и PrometheusRule (с лейблом release: observability для интеграции со стеком Лабы 2).
ServiceMonitor заменил собой ручной сбор метрик через additionalScrapeConfigs (который собирал метрики через ClusterIP и пропускал данные при нескольких репликах). Теперь Prometheus собирает метрики с каждого пода api отдельно, что дает полную и точную картину.
### Обоснование 3-х алертов:
- `ShopApiHighErrorRate` (severity: critical): sum(rate(http_requests_total{code=~"5.."}[1m])) > 0. Сигнализирует о появлении любых ошибок 5xx. Это прямой индикатор недоступности БД или внутренних сбоев критического пути.
  
- `ShopApiHighLatency` (severity: warning): histogram_quantile(0.95, ...) > 0.3. Напрямую защищает SLA (p95 < 300 мс), сформулированный в Части 8. Срабатывает при деградации производительности под нагрузкой.
  
- `ShopPodsPending` (severity: warning): sum(kube_pod_status_phase{phase="Pending"}) > 0. Сигнализирует о нехватке ресурсов на нодах и проблемах Capacity Planning, с которыми мы сталкивались в Части 6. Важно, что этот алерт использует системную метрику kube-state-metrics, не зависящую от сетевых запросов.

<img width="974" height="413" alt="image" src="https://github.com/user-attachments/assets/9e3ad799-1286-4a60-aa89-00e70164a290" />





