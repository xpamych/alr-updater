# Автоматическая генерация плагинов

ALR-updater поддерживает автоматическую генерацию плагинов для пакетов, размещённых на GitHub. Система может автоматически обнаруживать пакеты без плагинов и создавать для них соответствующие .star файлы.

## Возможности

### Автоматическое обнаружение
- ✅ Сканирование репозиториев для поиска пакетов без плагинов
- ✅ Определение GitHub репозиториев из файлов alr.sh
- ✅ Классификация пакетов по типам (binary, source, git)
- ✅ Генерация подходящих шаблонов плагинов

### Поддерживаемые типы пакетов
- **Binary packages (-bin)**: AppImage, tar.gz, zip архивы
- **Source packages**: tar.gz архивы с исходным кодом
- **Library packages**: C++ библиотеки, фреймворки
- **Tool packages**: Утилиты командной строки

### Автоматическое определение
- 📦 **Версии**: Извлечение из тегов GitHub (v1.2.3, release-1.2.3)
- 🔗 **URL скачивания**: Поиск подходящих assets в релизах
- 📅 **Расписание**: Автоматический выбор частоты проверок
- 🔍 **Asset паттерны**: Распознавание типов файлов

## Использование

### 1. Встроенная генерация

```bash
# Генерация всех недостающих плагинов
./alr-updater --generate-plugins

# С пользовательскими путями
./alr-updater --generate-plugins \
  --config=/path/to/config.toml \
  --plugin-dir=/path/to/plugins/
```

### 2. Анализ репозитория

```bash
# Сборка анализатора
make analyze-repo

# Анализ репозитория (табличный формат)
./analyze-repo --repo=alr-repo

# JSON формат для программной обработки
./analyze-repo --repo=alr-repo --format=json

# Анализ + генерация недостающих плагинов
./analyze-repo --repo=alr-repo --generate
```

### 3. Через Makefile

```bash
# Анализ репозитория
make analyze

# Генерация недостающих плагинов
make generate-missing

# Анализ в JSON формате
make analyze-json
```

## Примеры вывода

### Табличный анализ
```
📊 Анализ репозитория
═══════════════════════════════════════════════════════════════════════════════════
📦 Всего пакетов: 67
✅ С плагинами: 48
❌ Без плагинов: 19
🤖 Можно сгенерировать: 15

🎯 Пакеты для автогенерации:
─────────────────────────────────────────────────────────────────────────────────
📦 telegram-desktop-bin    │ telegramdesktop/tdesktop
📦 obsidian-bin           │ obsidianmd/obsidian-releases
📦 yarn                   │ yarnpkg/yarn
📦 electron-bin           │ electron/electron
```

### JSON анализ
```json
[
  {
    "package_name": "telegram-desktop-bin",
    "version": "5.0.1",
    "github_repo": "telegramdesktop/tdesktop",
    "sources": ["https://github.com/telegramdesktop/tdesktop/..."],
    "description": "Official Telegram Desktop client",
    "package_type": "github_release",
    "has_plugin": false,
    "can_generate_plugin": true
  }
]
```

## Структура сгенерированных плагинов

Сгенерированные плагины включают:

```python
# Repository: alr-repo
# Auto-generated plugin for package-name

REPO = "alr-repo"

def check_package_name():
    """Проверка обновлений для package-name с автоматическим обновлением checksums"""
    # Автоматическое определение версий
    # GitHub API интеграция  
    # Поиск подходящих assets
    # Обновление хеш-сумм
    # Автоматические коммиты

# Умное расписание на основе типа пакета
run_every.day(check_package_name)
```

## Конфигурация

### Специальные случаи

Можно настроить специальные правила для пакетов:

```go
// internal/generator/plugin_generator.go

func (pg *PluginGenerator) detectBinaryPackage(packageName string, detected DetectedPackage) DetectedPackage {
    switch {
    case strings.Contains(packageName, "telegram"):
        detected.AssetPattern = `tsetup\..*\.tar\.xz`
        detected.URLTemplate = "https://github.com/telegramdesktop/tdesktop/releases/download/{tag}/tsetup.{version}.tar.xz"
    case strings.Contains(packageName, "obsidian"):
        detected.AssetPattern = `.*\.AppImage$`
    // ... другие специальные случаи
    }
}
```

### Расписание проверок

Автоматически определяется на основе типа пакета:

- **Binary packages**: Каждые 6 часов
- **Library packages**: Каждую неделю  
- **Tool packages**: Каждый день

## Ограничения

### Текущие ограничения
- ❌ Только GitHub репозитории
- ❌ Требует стандартную структуру релизов
- ❌ Не поддерживает сложные схемы версионирования

### Планируемые улучшения
- 🔄 Поддержка GitLab, Gitea
- 🔄 Поддержка PyPI, npm, crates.io
- 🔄 Веб-интерфейс для настройки
- 🔄 Валидация сгенерированных плагинов

## Отладка

### Проверка сгенерированного плагина

```bash
# Проверка синтаксиса
./alr-updater --plugin-dir=./plugins --config=test-config.toml --debug

# Тестовый запуск
./alr-updater --now --debug
```

### Логирование
```bash
# Включить отладочные сообщения
./alr-updater --debug --generate-plugins
```

## Вклад в развитие

Для улучшения системы генерации:

1. **Добавление новых типов пакетов** в `detectPackageType()`
2. **Улучшение паттернов** в `detectBinaryPackage()`
3. **Новые источники** кроме GitHub
4. **Тестирование** на реальных пакетах

См. также:
- [Создание плагинов вручную](manual-plugins.md)
- [API документация](api.md)
- [Конфигурация](configuration.md)