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


##Часть 2

Для сбора и просмотра логов в Kubernetes был развёрнут стек Loki + Alloy. Loki используется как хранилище логов, а Alloy работает как агент сбора логов на узлах кластера.

Alloy был запущен в виде DaemonSet, поэтому его экземпляр работает на каждом узле Kubernetes-кластера. Он обнаруживает логи Kubernetes Pod'ов и отправляет их в Loki.

Loki был настроен в режиме SingleBinary с хранением данных в файловой системе. В Grafana был добавлен Loki как дополнительный источник данных. После этого логи Pod'ов стали доступны непосредственно в интерфейсе Grafana через раздел Explore.

Для проверки работы логирования был использован endpoint /fail приложения. При обращении к нему приложение возвращает ошибку и записывает соответствующее событие в JSON-лог.

Например, для генерации нескольких ошибок использовалась команда:

for i in {1..10}; do
  curl -s http://127.0.0.1:8080/fail >/dev/null
done

<img width="688" height="373" alt="image" src="https://github.com/user-attachments/assets/b9448a85-bc4d-4c5b-9259-c7303d215d2b" />
