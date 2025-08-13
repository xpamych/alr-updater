# ALR-Updater

Автоматизированный бот для проверки обновлений пакетов и их отправки в репозиторий [alr-repo](https://gitea.plemya-x.ru/Plemya-x/alr-repo).

---

## Описание

ALR-Updater — это модульный бот, предназначенный для автоматического отслеживания обновлений программного обеспечения и автоматического обновления пакетов в репозитории ALR. Система основана на плагинах написанных на языке [Starlark](https://github.com/bazelbuild/starlark), что обеспечивает гибкость и расширяемость.

### Принцип работы

Бот использует систему плагинов для мониторинга различных источников обновлений:

- **GitHub API** — для отслеживания релизов проектов на GitHub
- **API разработчиков** — для проверки обновлений через официальные API
- **Веб-хуки** — для мгновенного получения уведомлений о новых версиях
- **RSS/Atom ленты** — для мониторинга блогов и сайтов проектов

Каждый плагин может:
- Запускаться по расписанию (например, каждый час или день)
- Обрабатывать входящие веб-хуки
- Использовать постоянное хранилище ключ-значение для отслеживания состояния
- Автоматически обновлять файлы `alr.sh` в репозитории

### Примеры работы

**Discord-bin**: Плагин каждый час опрашивает API Discord для получения ссылки на последнюю версию. При обнаружении изменений он извлекает номер версии из URL и обновляет скрипт сборки.

**NodeJS**: Плагин отслеживает релизы на GitHub через API, проверяет доступность новых версий и автоматически обновляет зависимости и ссылки на исходный код.

---

## Установка

### Требования

- Go 1.20 или новее
- Git
- Доступ к репозиторию alr-repo

### Сборка из исходного кода

```bash
# Клонирование репозитория
git clone https://gitea.plemya-x.ru/Plemya-x/ALR-updater.git
cd ALR-updater

# Сборка
go build -o alr-updater main.go

# Установка (опционально)
sudo cp alr-updater /usr/local/bin/
```

---

## Настройка

### 1. Конфигурационный файл

Создайте конфигурационный файл на основе примера:

```bash
sudo mkdir -p /etc/alr-updater
sudo cp alr-updater.example.toml /etc/alr-updater/config.toml
```

Отредактируйте `/etc/alr-updater/config.toml`:

```toml
[git]
  repoURL = "https://gitea.plemya-x.ru/Plemya-x/alr-repo.git"
  repoDir = "/etc/alr-updater/repo"
  
  [git.commit]
    name = "ALR Updater Bot"
    email = "bot@example.com"
  
  [git.credentials]
    username = "your-username"
    password = "your-token"  # Используйте персональный токен доступа

[webhook]
  pwd_hash = "$2a$10$..." # Сгенерируйте командой: alr-updater -g
```

### 2. Генерация хеша пароля для веб-хуков

```bash
alr-updater -g
# Введите пароль и скопируйте полученный хеш в конфигурацию
```

### 3. Создание директории для плагинов

```bash
sudo mkdir -p /etc/alr-updater/plugins
```

---

## Создание плагинов

### Структура плагина

Плагины представляют собой файлы `.star` с кодом на языке Starlark. Пример базового плагина:

```python
# /etc/alr-updater/plugins/discord-bin.star

def check_discord_updates():
    """Проверка обновлений Discord"""
    
    # Получение текущей версии из пакета
    current_content = updater.get_package_file("discord-bin", "alr.sh")
    current_version = regex.find(r"version=([0-9.]+)", current_content)
    
    # Проверка новой версии через API
    response = http.get("https://discord.com/api/download/stable?platform=linux&format=tar.gz")
    download_url = response.url
    new_version = regex.find(r"/([0-9.]+)/discord", download_url)
    
    if new_version != current_version:
        log.info("Найдена новая версия Discord: " + new_version)
        
        # Обновление файла пакета
        new_content = regex.replace(
            current_content,
            r"version=[0-9.]+",
            "version=" + new_version
        )
        
        updater.write_package_file("discord-bin", "alr.sh", new_content)
        updater.push_changes("discord-bin: обновление до версии " + new_version)
    else:
        log.info("Discord уже актуален: " + current_version)

# Запуск проверки каждый час
run_every.hour(check_discord_updates)
```

### Плагин для GitHub пакетов

```python
# /etc/alr-updater/plugins/github-packages.star

def update_github_package(package_name, repo_path, version_prefix="v"):
    """Универсальная функция для обновления GitHub пакетов"""
    
    # Получение последнего релиза
    api_url = "https://api.github.com/repos/" + repo_path + "/releases/latest"
    response = http.get(api_url)
    release_data = response.json()
    
    latest_version = release_data["tag_name"]
    if latest_version.startswith(version_prefix):
        latest_version = latest_version[len(version_prefix):]
    
    # Получение текущей версии
    current_content = updater.get_package_file(package_name, "alr.sh")
    current_version = regex.find(r"version='([^']+)'", current_content)
    
    if latest_version != current_version:
        log.info("Обновление " + package_name + ": " + current_version + " -> " + latest_version)
        
        # Обновление версии в файле
        new_content = regex.replace(
            current_content,
            r"version='[^']+'",
            "version='" + latest_version + "'"
        )
        
        # Обновление ссылки на архив
        new_content = regex.replace(
            new_content,
            r"https://github\.com/" + repo_path + "/archive/refs/tags/[^)]+",
            "https://github.com/" + repo_path + "/archive/refs/tags/" + version_prefix + latest_version + ".tar.gz"
        )
        
        updater.write_package_file(package_name, "alr.sh", new_content)
        updater.push_changes(package_name + ": обновление до версии " + latest_version)

def check_fastfetch():
    update_github_package("fastfetch", "fastfetch-cli/fastfetch")

def check_nodejs():
    update_github_package("nodejs", "nodejs/node")

# Запуск проверок
run_every.day(check_fastfetch)
run_every.day(check_nodejs)
```

### Плагин для Python пакетов

```python
# /etc/alr-updater/plugins/python-packages.star

def update_python_package(package_name, pypi_name=None):
    """Обновление Python пакетов через PyPI API"""
    
    if not pypi_name:
        pypi_name = package_name.replace("python3-", "").replace("-", "_")
    
    # Запрос к PyPI API
    api_url = "https://pypi.org/pypi/" + pypi_name + "/json"
    response = http.get(api_url)
    
    if response.status_code != 200:
        log.error("Не удалось получить информацию о пакете " + pypi_name)
        return
    
    data = response.json()
    latest_version = data["info"]["version"]
    
    # Получение текущей версии
    current_content = updater.get_package_file(package_name, "alr.sh")
    current_version = regex.find(r"version='([^']+)'", current_content)
    
    if latest_version != current_version:
        log.info("Обновление " + package_name + ": " + current_version + " -> " + latest_version)
        
        # Обновление версии
        new_content = regex.replace(
            current_content,
            r"version='[^']+'",
            "version='" + latest_version + "'"
        )
        
        updater.write_package_file(package_name, "alr.sh", new_content)
        updater.push_changes(package_name + ": обновление до версии " + latest_version)

# Регистрация пакетов для проверки
python_packages = [
    "python3-rich",
    "python3-poetry-core",
    "python3-wheel",
    "python3-setuptools-rust"
]

def check_python_packages():
    for package in python_packages:
        update_python_package(package)

# Запуск еженедельно
run_every.week(check_python_packages)
```

---

## Запуск

### Ручной запуск

```bash
# Запуск с конфигурацией по умолчанию
alr-updater

# Запуск с кастомной конфигурацией
alr-updater -c /path/to/config.toml

# Запуск с другой директорией плагинов
alr-updater -p /path/to/plugins

# Включение отладочного режима
alr-updater -D
```

### Systemd сервис

Создайте файл `/etc/systemd/system/alr-updater.service`:

```ini
[Unit]
Description=ALR Updater Service
After=network.target

[Service]
Type=simple
User=alr-updater
Group=alr-updater
ExecStart=/usr/local/bin/alr-updater
Restart=always
RestartSec=30
Environment=HOME=/var/lib/alr-updater

[Install]
WantedBy=multi-user.target
```

Создайте пользователя и запустите сервис:

```bash
sudo useradd -r -s /bin/false alr-updater
sudo mkdir -p /var/lib/alr-updater
sudo chown alr-updater:alr-updater /var/lib/alr-updater
sudo chown -R alr-updater:alr-updater /etc/alr-updater

sudo systemctl daemon-reload
sudo systemctl enable alr-updater
sudo systemctl start alr-updater
```

### Docker

```dockerfile
FROM golang:1.20-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o alr-updater main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates git
WORKDIR /root/
COPY --from=builder /app/alr-updater .
COPY alr-updater.example.toml /etc/alr-updater/config.toml
CMD ["./alr-updater"]
```

---

## Настройка веб-хуков

### GitHub веб-хуки

1. Перейдите в настройки репозитория → Webhooks
2. Добавьте новый веб-хук:
   - **URL**: `http://your-server:8080/webhook/github`
   - **Content type**: `application/json`
   - **Events**: `Releases`
3. Добавьте секрет, соответствующий паролю в конфигурации

### Обработка веб-хуков в плагине

```python
# /etc/alr-updater/plugins/webhooks.star

def handle_github_release(data):
    """Обработка веб-хука GitHub релиза"""
    
    if data["action"] != "published":
        return
    
    repo_name = data["repository"]["full_name"]
    tag_name = data["release"]["tag_name"]
    
    # Маппинг репозиториев к пакетам
    repo_to_package = {
        "fastfetch-cli/fastfetch": "fastfetch",
        "nodejs/node": "nodejs"
    }
    
    package_name = repo_to_package.get(repo_name)
    if package_name:
        log.info("Получен веб-хук для " + package_name + " версии " + tag_name)
        update_github_package(package_name, repo_name)

# Регистрация обработчика веб-хуков
webhook.github(handle_github_release, password="your-webhook-password")
```

---

## Мониторинг и логирование

### Просмотр логов

```bash
# Логи systemd сервиса
sudo journalctl -u alr-updater -f

# Логи в файл
alr-updater 2>&1 | tee /var/log/alr-updater.log
```

### Уведомления в Telegram

```python
# В плагине
def send_telegram_notification(message):
    telegram_bot_token = "YOUR_BOT_TOKEN"
    chat_id = "YOUR_CHAT_ID"
    
    url = "https://api.telegram.org/bot" + telegram_bot_token + "/sendMessage"
    data = {
        "chat_id": chat_id,
        "text": message,
        "parse_mode": "Markdown"
    }
    
    http.post(url, json=data)

# Использование в функции обновления
def update_package_with_notification(package_name, new_version):
    # ... логика обновления ...
    
    message = "🔄 *ALR-Updater*\n\n" \
            + "Обновлен пакет: `" + package_name + "`\n" \
            + "Новая версия: `" + new_version + "`"
    
    send_telegram_notification(message)
```

---

## Расширенные возможности

### Переменные окружения

Вы можете использовать переменные окружения вместо файла конфигурации:

```bash
export GIT_REPO_URL="https://gitea.plemya-x.ru/Plemya-x/alr-repo.git"
export GIT_REPO_DIR="/tmp/alr-repo"
export GIT_COMMIT_NAME="ALR Updater"
export GIT_COMMIT_EMAIL="bot@example.com"
export GIT_CREDENTIALS_USERNAME="username"
export GIT_CREDENTIALS_PASSWORD="token"
export WEBHOOK_PASSWORD_HASH="$2a$10$..."

alr-updater -E  # Использование переменных окружения
```

### Тестирование плагинов

```bash
# Создайте тестовый плагин
cat > /tmp/test-plugin.star << 'EOF'
def test_function():
    log.info("Тестовый плагин работает!")
    
    # Тест получения файла пакета
    content = updater.get_package_file("fastfetch", "alr.sh")
    log.info("Получен файл пакета, размер: " + str(len(content)))

run_every.minute(test_function)
EOF

# Запуск с тестовым плагином
alr-updater -p /tmp -D
```

---

## Troubleshooting

### Частые проблемы

**1. Ошибка аутентификации Git**
```
Error: authentication required
```
Решение: Проверьте правильность токена доступа и имени пользователя в конфигурации.

**2. Плагин не выполняется**
```
Error executing starlark file
```
Решение: Проверьте синтаксис Starlark кода, включите отладочный режим (`-D`).

**3. Не удается получить файл пакета**
```
Error: no such file or directory
```
Решение: Убедитесь, что репозиторий успешно клонирован и пакет существует.

### Отладка

```bash
# Включение подробного логирования
alr-updater -D

# Проверка состояния базы данных
ls -la /etc/alr-updater/db

# Проверка Git репозитория
cd /etc/alr-updater/repo && git status
```

---

## Участие в разработке

1. Форкните репозиторий
2. Создайте ветку для новой функции
3. Внесите изменения
4. Создайте Pull Request

### Структура проекта

```
ALR-updater/
├── main.go                 # Основной файл приложения
├── internal/
│   ├── builtins/          # Встроенные функции для Starlark
│   ├── config/            # Конфигурация
│   └── convert/           # Утилиты конвертации
├── alr-updater.example.toml # Пример конфигурации
└── README.md              # Эта документация
```

---

## Лицензия

Этот проект распространяется под лицензией GPL v3. См. файл LICENSE для подробностей.
