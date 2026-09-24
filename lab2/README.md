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

