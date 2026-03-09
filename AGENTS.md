# AGENTS.md — ALR-Updater

Файл с описанием проекта для AI ассистентов. Содержит архитектурные детали, соглашения о коде и инструкции для разработки.

---

## Обзор проекта

**ALR-Updater** — автоматизированный бот для отслеживания обновлений пакетов и их автоматической отправки в репозитории ALR (Any Linux Repository). Система основана на плагинах, написанных на языке Starlark (Python-подобный язык от Google), что обеспечивает гибкость и расширяемость.

### Основные возможности

- Проверка обновлений пакетов по расписанию (каждую минуту/час/день/неделю)
- Автоматическое обновление версий, релизов и хеш-сумм в файлах пакетов
- Поддержка множественных репозиториев Git
- Веб-хуки для мгновенных уведомлений о новых версиях
- Автоматическая генерация плагинов для GitHub-репозиториев
- Логирование в файл с ротацией

---

## Технологический стек

- **Язык**: Go 1.20+
- **Скриптинг**: Starlark (go.starlark.net)
- **База данных**: BoltDB (go.etcd.io/bbolt) — key-value хранилище
- **Git операции**: go-git/v5 (чистый Go, без зависимости от git CLI)
- **Конфигурация**: TOML (pelletier/go-toml/v2) + переменные окружения
- **Логирование**: go.elara.ws/logger
- **HTTP клиент**: стандартный net/http с поддержкой редиректов
- **Аутентификация**: bcrypt для хеширования паролей веб-хуков

### Зависимости (go.mod)

```
github.com/PuerkitoBio/goquery      # HTML парсинг
github.com/caarlos0/env/v8          # Переменные окружения
github.com/go-git/go-git/v5         # Git операции
github.com/pelletier/go-toml/v2     # TOML конфигурация
github.com/spf13/pflag              # CLI флаги
go.etcd.io/bbolt                    # Embedded БД
go.starlark.net                     # Starlark интерпретатор
```

---

## Структура проекта

```
ALR-updater/
├── main.go                          # Точка входа, инициализация
├── internal/
│   ├── builtins/                    # Встроенные модули Starlark
│   │   ├── register.go              # Регистрация всех модулей
│   │   ├── http.go                  # HTTP запросы (GET/POST/PUT/HEAD)
│   │   ├── updater.go               # Git операции, работа с файлами
│   │   ├── run_every.go             # Планировщик задач
│   │   ├── regex.go                 # Регулярные выражения
│   │   ├── checksum.go              # SHA256 хеш-суммы
│   │   ├── store.go                 # BoltDB хранилище
│   │   ├── log.go                   # Логирование из плагинов
│   │   ├── html.go                  # HTML парсинг
│   │   ├── reader.go                # Чтение данных для Starlark
│   │   └── utils.go                 # Вспомогательные функции
│   ├── config/
│   │   └── config.go                # Структуры конфигурации
│   ├── generator/
│   │   └── plugin_generator.go      # Автогенерация плагинов
│   ├── logger/
│   │   └── file_logger.go           # Ротация логов
│   ├── convert/
│   │   └── convert.go               # Конвертация типов
│   └── permissions/
│       └── permissions.go           # Управление правами доступа
├── cmd/
│   └── analyze-repo/
│       └── main.go                  # Утилита анализа репозитория
├── plugins/                         # Starlark плагины (*.star)
├── scripts/
│   └── install.sh                   # Скрипт установки
├── docs/
│   └── plugin-generation.md         # Документация по генерации
├── Makefile                         # Команды сборки
├── go.mod, go.sum                   # Go модули
├── alr-updater.example.toml         # Пример конфигурации
├── template.nomad                   # Nomad шаблон деплоя
└── .woodpecker.yml                  # CI/CD конфигурация
```

---

## Сборка и запуск

### Команды Makefile

```bash
# Сборка всех бинарников
make build

# Только основной бинарник
make alr-updater

# Утилита анализа репозитория
make analyze-repo

# Установка в /usr/local/bin
make install

# Запуск тестов
make test

# Очистка
make clean

# Анализ репозитория
make analyze          # Табличный формат
make analyze-json     # JSON формат

# Генерация плагинов
make generate-plugins      # Через alr-updater
make generate-missing      # Через analyze-repo
```

### Прямая сборка Go

```bash
# Сборка с CGO (требуется для BoltDB)
CGO_ENABLED=1 go build -o alr-updater main.go

# Сборка analyze-repo
cd cmd/analyze-repo && go build -o ../../analyze-repo .
```

### Флаги запуска

```bash
./alr-updater -c /etc/alr-updater/config.toml    # Путь к конфигу
./alr-updater -p /etc/alr-updater/plugins        # Путь к плагинам
./alr-updater -d /var/lib/alr-updater/db         # Путь к БД
./alr-updater -a :8080                           # Адрес HTTP сервера
./alr-updater -D                                 # Режим отладки
./alr-updater -g                                 # Генерация хеша пароля
./alr-updater --now                              # Немедленный запуск всех проверок
./alr-updater --generate-plugins                 # Автогенерация плагинов
./alr-updater -E                                 # Использовать переменные окружения
```

---

## Архитектура

### Жизненный цикл приложения

1. **Инициализация**:
   - Парсинг CLI флагов
   - Загрузка конфигурации (TOML или env)
   - Настройка логирования
   - Открытие BoltDB

2. **Подготовка директорий**:
   - Создание `/var/lib/alr-updater` (БД)
   - Создание `/var/cache/alr-updater` (репозитории)
   - Создание `/etc/alr-updater/plugins`

3. **Клонирование репозиториев**:
   - Для каждого репозитория в конфиге
   - Git clone если не существует
   - Настройка sharedRepository

4. **Загрузка плагинов**:
   - Поиск всех `*.star` файлов
   - Парсинг `# Repository: repo-name` комментария
   - Регистрация встроенных модулей
   - ExecFile каждого плагина

5. **Запуск сервера**:
   - HTTP сервер на порту 8080 (по умолчанию)
   - Обработка веб-хуков

### Система плагинов

Каждый плагин — это `.star` файл с обязательным комментарием:

```python
# Repository: alr-repo

REPO = "alr-repo"

def check_package():
    # Логика проверки обновлений
    pass

run_every.day(check_package)
```

#### Доступные модули в Starlark

| Модуль | Функции | Описание |
|--------|---------|----------|
| `run_every` | `minute(fn, count)`, `hour(fn, count)`, `day(fn)`, `week(fn)` | Планировщик |
| `http` | `get(url)`, `post(url, body)`, `put(url, body)`, `head(url)` | HTTP запросы |
| `updater` | `pull(repo)`, `push_changes(repo, msg)`, `get_package_file(repo, pkg, file)`, `write_package_file(repo, pkg, file, content)`, `update_checksums(repo, pkg, file, content, checksums)` | Git операции |
| `regex` | `find(pattern, text)`, `replace(text, pattern, replacement)` | Регулярные выражения |
| `checksum` | `calculate_sha256(url)` | Вычисление хеш-сумм |
| `store` | `get(key)`, `set(key, value)` | Хранилище BoltDB |
| `log` | `info(msg)`, `warn(msg)`, `error(msg)` | Логирование |
| `json` | `encode(obj)`, `decode(str)` | JSON операции |
| `html` | `parse(html)` | HTML парсинг |
| `utils` | `semver_compare(v1, v2)` | Утилиты |
| `register_webhook` | `register_webhook(fn, secure=True)` | Веб-хуки |

### Конфигурация

Файл `/etc/alr-updater/config.toml`:

```toml
# Базовая директория для репозиториев
reposBaseDir = "/var/cache/alr-updater"

# Репозитории
[repositories.alr-repo]
  repoURL = "https://gitea.plemya-x.ru/Plemya-x/alr-repo.git"
  [repositories.alr-repo.commit]
    name = "ALR Bot"
    email = "bot@example.com"
  [repositories.alr-repo.credentials]
    username = "username"
    password = "token"

[repositories.alr-LG]
  repoURL = "https://git.linux-gaming.ru/Linux-Gaming/alr-LG.git"
  # ... аналогично

[webhook]
  pwd_hash = "$2a$10$..."  # bcrypt хеш

[logging]
  enable_file = true
  log_file = "/var/log/alr-updater.log"
  max_size = 104857600  # 100MB

[github]
  token = "ghp_..."  # Для API запросов
```

---

## Соглашения о коде

### Стиль кода Go

- **Лицензионный заголовок**: Все файлы должны содержать GPL v3 заголовок
- **Комментарии**: На русском языке для бизнес-логики, на английском для технических деталей
- **Именование**: CamelCase для экспортируемых, camelCase для внутренних
- **Организация**: `internal/` для приватного кода, `cmd/` для утилит

### Паттерн для встроенных модулей

```go
// Файл: internal/builtins/example.go
package builtins

import "go.starlark.net/starlark"

func exampleModule(opts *Options) *starlarkstruct.Module {
    return &starlarkstruct.Module{
        Name: "example",
        Members: starlark.StringDict{
            "function": starlark.NewBuiltin("example.function", exampleFunc),
        },
    }
}

func exampleFunc(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
    // Реализация
    return starlark.None, nil
}
```

### Работа с Git репозиториями

- Используется **go-git** (чистый Go)
- Каждый репозиторий имеет свой мьютекс (`repoMtxMap`)
- Автоматическое исправление прав после операций
- Поддержка `sharedRepository = group`

### Правила обновления пакетов

Плагины должны обновлять **только три поля**:

1. `version` — версия пакета (всегда в одинарных кавычках `'')
2. `release` — номер релиза (сбрасывается в `'1'` при изменении версии)
3. `checksums` — хеш-суммы файлов

**НЕ изменяются**: `sources`, `description`, `depends`, `conflicts`

---

## Тестирование

### Запуск тестов

```bash
# Все тесты
go test -v ./...

# С покрытием
go test -cover ./...
```

### Ручное тестирование плагинов

```bash
# Создать тестовый плагин
cat > /tmp/test.star << 'EOF'
def test():
    log.info("Test plugin works!")
    content = updater.get_package_file("alr-repo", "fastfetch", "alr.sh")
    log.info("File size: " + str(len(content)))

run_every.minute(test)
EOF

# Запуск с тестовым плагином
./alr-updater -p /tmp -D --now
```

---

## Деплой и CI/CD

### Woodpecker CI (.woodpecker.yml)

```yaml
pipeline:
  release:
    image: goreleaser/goreleaser
    commands:
      - goreleaser release
    when:
      event: tag
  
  deploy:
    image: loq9/drone-nomad
    settings:
      addr: http://192.168.100.62:4646
      template: template.nomad
```

### Nomad деплой (template.nomad)

- Docker driver
- Traefik для маршрутизации
- Артефакты скачиваются из Gitea releases

### Локальная установка

```bash
sudo ./scripts/install.sh
# Создаёт:
# - Пользователя alr-updater
# - Директории /etc/alr-updater, /var/lib/alr-updater, /var/cache/alr-updater
# - Systemd сервис
# - Права с setgid битом
```

---

## Безопасность

### Права доступа

- Сервис работает от пользователя `alr-updater` (группа `wheel`)
- Директории с `setgid` битом (2775)
- Конфигурация только для чтения во время работы
- Лог-файл доступен для записи

### Systemd hardening

```ini
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/alr-updater /var/cache/alr-updater /var/log/alr-updater.log
ReadOnlyPaths=/etc/alr-updater
```

### Веб-хуки

- Аутентификация через Basic Auth или Bearer токен
- bcrypt хеширование паролей
- Возможность создания незащищённых хуков (не рекомендуется)

---

## Генерация плагинов

### Автоматическое обнаружение пакетов

```bash
# Генерация всех недостающих плагинов
./alr-updater --generate-plugins

# Или через analyze-repo
./analyze-repo --repo=alr-repo --generate
```

### Алгоритм генерации

1. Сканирование репозитория на наличие `alr.sh` файлов
2. Анализ `sources` и `homepage` для определения GitHub репозитория
3. Определение типа пакета (binary, source, library)
4. Выбор расписания проверок
5. Генерация `.star` файла из шаблона

### Поддерживаемые типы

- **-bin пакеты**: AppImage, tar.gz, zip (каждые 6 часов)
- **Исходные пакеты**: tar.gz с GitHub releases (каждый день)
- **Библиотеки**: C++ libs, cmake modules (каждую неделю)

---

## Отладка

### Просмотр логов

```bash
# Systemd журнал
journalctl -u alr-updater -f
journalctl -u alr-updater -n 100

# Файл логов
tail -f /var/log/alr-updater.log
grep ERROR /var/log/alr-updater.log
```

### Ручной запуск проверок

```bash
sudo -u alr-updater /usr/local/bin/alr-updater --now
```

### Отладочный режим

```bash
./alr-updater -D  # Включает debug логирование
```

---

## Известные ограничения Starlark

- ❌ Нет `try-except` блоков — используйте условные проверки
- ❌ Нет `.get()` у словарей с значением по умолчанию
- ❌ Нет функции `type()` для проверки типов
- ❌ Строгая типизация — нельзя смешивать строки и числа
- ✅ Есть `json` модуль для парсинга API ответов
- ✅ Есть `html` модуль для парсинга веб-страниц

---

## Полезные ссылки

- Репозиторий: https://gitea.plemya-x.ru/Plemya-x/ALR-updater
- ALR репозитории:
  - https://gitea.plemya-x.ru/Plemya-x/alr-repo
  - https://gitea.plemya-x.ru/Plemya-x/alr-default
  - https://git.linux-gaming.ru/Linux-Gaming/alr-LG
- Starlark spec: https://github.com/bazelbuild/starlark
- go-git docs: https://pkg.go.dev/github.com/go-git/go-git/v5
