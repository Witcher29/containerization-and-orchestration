## Часть 2

### Запуск сервиса api в собственном namespace. 
*sudo unshare --pid --mount --net --uts --ipc --user --map-root-user --fork --mount-proc ./api*
- Флаг pid задаёт процессу PID=1, т.е. думает сервис api думает, что стал root'ом.
- Флаг mount изолирует файловую системы
- Флаг net создаёт lo
- Флаг uts даёт возможность задать hostname в контейнере
- Флаг ipc выносит в своё пространство медпроцессное взаимодействие для контейнера
- Флаг user задаёт изолированное пространство имён пользователей и групп.
- Флаг map-root-user сделает так, что внутри контейнера процесс, выполняемый от имени root, в реальной машине будет выполняться от имени пользователя
- Флаг fork создаёт дочерний процесс, внутри которого и будет выполняться сервис api
- Флаг mount-proc нужен, чтобы внутри изолированной среды утилиты мониторинга не видели процессов за пределами контейнера

Запуск команды представлен на рисунке снизу:
<img width="928" height="67" alt="image" src="https://github.com/user-attachments/assets/64adf1b1-86bb-44e1-a14f-d41694660849" />
Для успешного запуска от не root пользователя пришлось снять защиту от несанкционированного создания пользовательских пространств имён:

*sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0*

В результате выполнения команды получаем процесс, находящийся в собственном namespace. Внутри этого namespace процесс сервиса api работает от имени root, также имеет собственную сеть, hostname (если задать отличный от оригинального) и область видимости средств мониторинга (ps aux, запущенный внутри namespace не показывает процессы хоста):

<img width="917" height="453" alt="image" src="https://github.com/user-attachments/assets/0199e5f8-dfc6-4756-8048-462f0f110d43" />

Демонстрация сети хоста для сравнения с сетью контейнера на предыдущем скриншоте:
<img width="797" height="275" alt="image" src="https://github.com/user-attachments/assets/7faa04df-ed73-469e-8fc2-75e69c466610" />

## Часть 4
### Ограничение с помощью привилегий
Для начала с помощью следующей команды запретим внутри нашего контейнера дополнительные возможности с помощью ограничения привилегий:

*unshare --pid --mount --net --uts --ipc --user --map-root-user --fork \
  --mount-proc \
  setpriv --inh-caps=-all --ambient-caps=-all --bounding-set=-all \
    ./api*

<img width="958" height="136" alt="image" src="https://github.com/user-attachments/assets/739f5a67-064a-40d1-9c6c-1a0c467de01b" />

Эта команда помимо оговорённого в части 2 запускает процесс с указанными привилегиями, в данном случае с указанием отсутствия привилегий (у всех атрибутов стоит выключение). setpriv не запускает отдельный процесс, а подменяет свой процессом api (execve).
- Атрибут inh-caps влияет на передачу набора привилегий процессам-потомкам при exeve
- Атрибут ambient-caps делает так, что никакой набор привилегий не сохранится при exeve
- Атрибут bounding-set в данном случае навсегда лишает процесс шанса на получение какой-то привелегии даже от root'а

На картинке снизу демонстрируется, что пользователь внутри процесса не может изменить время, потому что не имеет на это прав.

<img width="960" height="278" alt="image" src="https://github.com/user-attachments/assets/e44ad9d9-d5ab-4bdf-8ecc-76ef75ae09d8" />

### Блокировка системного вызова
С помощью команды ниже мы помимо вышеописанного запрещаем изнутри контейнера делать системный вызов, отвечающий за создание директории:
*systemd-run --user --wait --pty   -p SystemCallFilter="~mkdir"   -p SystemCallErrorNumber=EPERM   unshare --pid --mount --net --uts --ipc --user --map-root-user --fork   --mount-proc   setpriv --inh-caps=-all --ambient-caps=-all --bounding-set=-all /home/alexandr/Desktop/containerization-labs/lab1/api*

<img width="958" height="141" alt="image" src="https://github.com/user-attachments/assets/928cfdd3-6804-4988-b2fe-f953295d7fb0" />

- Флаг user запускает модуль от имени пользовательского менеджера systemd (user@UID.service), а не системного. Не нужен root. Настройки применяются в рамках сессии.
- Флаг wait задаёт ожидание выполнения команды
- Флаг pty подключает псевдотерминал (PTY) — чтобы вывод команды шёл в ваш терминал в реальном времени, как при обычном запуске.
- Seccomp фильтр SystemCallFilter="~mkdir" запрещает вызов mkdir
- SystemCallErrorNumber=EPERM указывает, что при вызове запрещённого вызова нужно вернуть ошибку Operation not permitted

При попытке создать папку получаем указанную ошибку:
<img width="956" height="416" alt="image" src="https://github.com/user-attachments/assets/c40196a9-68f6-4aab-a10b-881ff923cc91" />



