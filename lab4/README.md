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
