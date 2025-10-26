package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"gitea.plemya-x.ru/Plemya-x/ALR-updater/internal/config"
	"gitea.plemya-x.ru/Plemya-x/ALR-updater/internal/permissions"
	"go.elara.ws/logger/log"
)

// PluginTemplate представляет шаблон плагина
type PluginTemplate struct {
	PackageName    string
	FunctionName   string
	Repository     string
	GitHubRepo     string
	AssetPattern   string
	URLTemplate    string
	URLPattern     string
	VersionPrefix  string
	Schedule       string
	ScheduleCalls  string
}

// PackageInfo содержит информацию о пакете из alr.sh
type PackageInfo struct {
	Name        string
	Version     string
	Sources     []string
	Description string
	Homepage    string
}

// DetectedPackage содержит автоматически определенную информацию о пакете
type DetectedPackage struct {
	PackageInfo
	GitHubRepo    string
	PackageType   string
	AssetPattern  string
	URLTemplate   string
	VersionPrefix string
}

const pluginTemplateStr = `# Repository: {{.Repository}}
# Auto-generated plugin for {{.PackageName}}

REPO = "{{.Repository}}"

def check_{{.FunctionName}}():
    """Проверка обновлений для {{.PackageName}} с автоматическим обновлением checksums"""
    package_name = "{{.PackageName}}"
    
    # Обновляем репозиторий перед проверкой
    updater.pull(REPO)
    
    # Получение текущей версии из пакета
    current_content = updater.get_package_file(REPO, package_name, "alr.sh")
    # Универсальное распознавание: без кавычек, одинарные кавычки, двойные кавычки
    current_version_matches = regex.find(r"version=['\"]?([0-9.]+)['\"]?", current_content)
    
    if not current_version_matches or len(current_version_matches) == 0:
        log.error("Не удалось найти текущую версию для " + package_name)
        return
    
    current_version = current_version_matches[0]
    
    # Проверка новой версии через GitHub API
    api_url = "https://api.github.com/repos/{{.GitHubRepo}}/releases/latest"
    response = http.get(api_url)
    if not response:
        log.error("Не удалось получить ответ от GitHub API для " + package_name)
        return
    
    release_data = response.json()
    if not release_data or "tag_name" not in release_data:
        log.error("Нет данных о релизе для " + package_name)
        return
    
    # Извлекаем версию из тега{{if .VersionPrefix}} (убираем '{{.VersionPrefix}}' если есть){{end}}
    tag_name = release_data["tag_name"]
    {{if .VersionPrefix}}new_version = tag_name[{{len .VersionPrefix}}:] if tag_name.startswith("{{.VersionPrefix}}") else tag_name{{else}}new_version = tag_name[1:] if tag_name.startswith("v") else tag_name{{end}}
    
    # Сравнение версий
    if new_version != current_version:
        log.info("Обнаружено обновление " + package_name + ": " + current_version + " -> " + new_version)
        
        {{if .URLTemplate}}# URL для скачивания по шаблону
        download_url = "{{.URLTemplate}}".replace("{tag}", tag_name).replace("{version}", new_version){{else if .AssetPattern}}# Находим URL для файла по паттерну
        download_url = ""
        if "assets" in release_data:
            for asset in release_data["assets"]:
                asset_matches = regex.find(r"{{.AssetPattern}}", asset["name"])
                if asset_matches and len(asset_matches) > 0:
                    download_url = asset["browser_download_url"]
                    break
        
        if not download_url:
            log.error("Не найден подходящий asset для " + package_name)
            return{{else}}# Ищем первый подходящий asset
        download_url = ""
        if "assets" in release_data and len(release_data["assets"]) > 0:
            download_url = release_data["assets"][0]["browser_download_url"]{{end}}
        
        # Обновление версии в файле (сохраняем формат кавычек)
        # Определяем какой формат используется
        if regex.find(r"version='[0-9.]+'", current_content):
            # Одинарные кавычки
            new_content = regex.replace(
                current_content,
                r"version='[0-9.]+'",
                "version='" + new_version + "'"
            )
        elif regex.find(r'version="[0-9.]+"', current_content):
            # Двойные кавычки
            new_content = regex.replace(
                current_content,
                r'version="[0-9.]+"',
                'version="' + new_version + '"'
            )
        else:
            # Без кавычек
            new_content = regex.replace(
                current_content,
                r"version=[0-9.]+",
                "version=" + new_version
            )
        
        {{if .URLPattern}}# Обновление URL загрузки в файле
        new_content = regex.replace(
            new_content,
            r"{{.URLPattern}}",
            download_url
        ){{end}}
        
        # Вычисление новой хеш-суммы
        log.info("Вычисление SHA256 для " + download_url)
        new_checksum = checksum.calculate_sha256(download_url)
        log.info("Новая хеш-сумма: " + new_checksum)
        
        # Обновление файла с новыми checksums
        updater.update_checksums(
            REPO,
            package_name, 
            "alr.sh", 
            new_content,
            [new_checksum]
        )
        
        # Коммит и push изменений
        updater.push_changes(REPO, package_name + ": обновление до версии " + new_version)
        
        log.info("Пакет " + package_name + " успешно обновлён до версии " + new_version + " с новой хеш-суммой")
    else:
        log.info("Пакет " + package_name + " уже имеет последнюю версию: " + current_version)

# {{.Schedule}}
{{.ScheduleCalls}}`

// PluginGenerator представляет генератор плагинов
type PluginGenerator struct {
	config      *config.Config
	template    *template.Template
	reposDir    string
	pluginsDir  string
}

// NewPluginGenerator создает новый генератор плагинов
func NewPluginGenerator(cfg *config.Config, pluginsDir string) (*PluginGenerator, error) {
	tmpl, err := template.New("plugin").Parse(pluginTemplateStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &PluginGenerator{
		config:     cfg,
		template:   tmpl,
		reposDir:   cfg.ReposBaseDir,
		pluginsDir: pluginsDir,
	}, nil
}

// ScanRepository сканирует репозиторий и находит пакеты без плагинов
func (pg *PluginGenerator) ScanRepository(repoName string) ([]DetectedPackage, error) {
	repoPath := filepath.Join(pg.reposDir, repoName)
	if _, err := os.Stat(repoPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("repository directory not found: %s", repoPath)
	}

	var packages []DetectedPackage

	err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Ищем файлы alr.sh
		if info.Name() == "alr.sh" {
			packageDir := filepath.Dir(path)
			packageName := filepath.Base(packageDir)

			// Пропускаем если плагин уже существует
			pluginFile := filepath.Join(pg.pluginsDir, packageName+".star")
			if _, err := os.Stat(pluginFile); err == nil {
				return nil
			}

			// Анализируем пакет
			pkgInfo, err := pg.analyzePackage(path)
			if err != nil {
				fmt.Printf("Warning: failed to analyze package %s: %v\n", packageName, err)
				return nil
			}

			detected := pg.detectPackageType(packageName, *pkgInfo)
			if detected.GitHubRepo != "" {
				packages = append(packages, detected)
			}
		}

		return nil
	})

	return packages, err
}

// analyzePackage анализирует файл alr.sh пакета
func (pg *PluginGenerator) analyzePackage(alrFile string) (*PackageInfo, error) {
	content, err := os.ReadFile(alrFile)
	if err != nil {
		return nil, err
	}

	info := &PackageInfo{
		Name: filepath.Base(filepath.Dir(alrFile)),
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Извлекаем версию
		if strings.HasPrefix(line, "version=") {
			versionMatch := regexp.MustCompile(`version=['"]?([^'"]+)['"]?`).FindStringSubmatch(line)
			if len(versionMatch) > 1 {
				info.Version = versionMatch[1]
			}
		}

		// Извлекаем источники
		if strings.Contains(line, "github.com") || strings.Contains(line, "https://") {
			urlMatch := regexp.MustCompile(`https://[^\s"']+`).FindAllString(line, -1)
			info.Sources = append(info.Sources, urlMatch...)
		}

		// Извлекаем описание
		if strings.HasPrefix(line, "desc=") {
			descMatch := regexp.MustCompile(`desc=['"]?([^'"]+)['"]?`).FindStringSubmatch(line)
			if len(descMatch) > 1 {
				info.Description = descMatch[1]
			}
		}

		// Извлекаем homepage
		if strings.HasPrefix(line, "homepage=") {
			homepageMatch := regexp.MustCompile(`homepage=['"]?([^'"]+)['"]?`).FindStringSubmatch(line)
			if len(homepageMatch) > 1 {
				info.Homepage = homepageMatch[1]
			}
		}
	}

	return info, nil
}

// detectPackageType определяет тип пакета и GitHub репозиторий
func (pg *PluginGenerator) detectPackageType(packageName string, info PackageInfo) DetectedPackage {
	detected := DetectedPackage{
		PackageInfo: info,
		PackageType: "unknown",
	}

	// Анализируем источники для GitHub репозиториев
	githubRegex := regexp.MustCompile(`github\.com/([^/]+/[^/]+)`)
	
	for _, source := range info.Sources {
		if match := githubRegex.FindStringSubmatch(source); len(match) > 1 {
			detected.GitHubRepo = match[1]
			detected.PackageType = "github_release"
			break
		}
	}

	// Если не найдено в источниках, проверяем homepage
	if detected.GitHubRepo == "" && info.Homepage != "" {
		if match := githubRegex.FindStringSubmatch(info.Homepage); len(match) > 1 {
			detected.GitHubRepo = match[1]
			detected.PackageType = "github_release"
		}
	}

	// Определяем паттерны для разных типов пакетов
	if strings.HasSuffix(packageName, "-bin") {
		detected = pg.detectBinaryPackage(packageName, detected)
	} else if strings.HasSuffix(packageName, "-git") {
		detected.PackageType = "git_commits"
	} else {
		detected = pg.detectSourcePackage(packageName, detected)
	}

	return detected
}

// detectBinaryPackage определяет паттерны для бинарных пакетов
func (pg *PluginGenerator) detectBinaryPackage(packageName string, detected DetectedPackage) DetectedPackage {
	switch {
	case strings.Contains(packageName, "telegram"):
		detected.AssetPattern = `tsetup\..*\.tar\.xz`
		detected.URLTemplate = "https://github.com/telegramdesktop/tdesktop/releases/download/{tag}/tsetup.{version}.tar.xz"
	case strings.Contains(packageName, "discord"):
		detected.AssetPattern = `.*\.tar\.gz$`
	case strings.Contains(packageName, "obsidian"):
		detected.AssetPattern = `.*\.AppImage$`
	case strings.Contains(packageName, "electron"):
		detected.AssetPattern = `.*-linux-x64\.zip$`
	case strings.Contains(packageName, "vscode") || strings.Contains(packageName, "code"):
		detected.AssetPattern = `.*\.tar\.gz$`
	default:
		// Общие паттерны для бинарных пакетов
		detected.AssetPattern = `.*linux.*\.(tar\.gz|tar\.xz|zip|AppImage)$`
	}

	return detected
}

// detectSourcePackage определяет паттерны для исходных пакетов
func (pg *PluginGenerator) detectSourcePackage(packageName string, detected DetectedPackage) DetectedPackage {
	// Для большинства исходных пакетов используем архив тега
	detected.URLTemplate = "https://github.com/{repo}/archive/{tag}.tar.gz"
	
	// Специальные случаи
	switch packageName {
	case "md4c":
		detected.VersionPrefix = "release-"
	case "steamcmd":
		detected.PackageType = "http_lastmodified"
	}

	return detected
}

// GeneratePlugin генерирует плагин для пакета
func (pg *PluginGenerator) GeneratePlugin(detected DetectedPackage, repoName string) error {
	functionName := strings.ReplaceAll(strings.ReplaceAll(detected.Name, "-", "_"), ".", "_")
	
	schedule := pg.determineSchedule(detected)
	scheduleCalls := pg.generateScheduleCalls(functionName, schedule)

	data := PluginTemplate{
		PackageName:   detected.Name,
		FunctionName:  functionName,
		Repository:    repoName,
		GitHubRepo:    detected.GitHubRepo,
		AssetPattern:  detected.AssetPattern,
		URLTemplate:   detected.URLTemplate,
		URLPattern:    pg.generateURLPattern(detected),
		VersionPrefix: detected.VersionPrefix,
		Schedule:      schedule,
		ScheduleCalls: scheduleCalls,
	}

	pluginFile := filepath.Join(pg.pluginsDir, detected.Name+".star")
	file, err := os.Create(pluginFile)
	if err != nil {
		return fmt.Errorf("failed to create plugin file: %w", err)
	}
	defer file.Close()

	err = pg.template.Execute(file, data)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Восстанавливаем права на файл плагина
	if err := permissions.FixFilePermissions(pluginFile); err != nil {
		log.Warn("Failed to fix plugin file permissions").Str("path", pluginFile).Err(err).Send()
	}

	fmt.Printf("✅ Generated plugin: %s\n", pluginFile)
	return nil
}

// determineSchedule определяет расписание проверки обновлений
func (pg *PluginGenerator) determineSchedule(detected DetectedPackage) string {
	switch {
	case strings.HasSuffix(detected.Name, "-bin"):
		return "Запуск проверки каждые 6 часов"
	case strings.Contains(detected.Name, "lib") || strings.Contains(detected.Name, "cmake"):
		return "Запуск проверки каждую неделю"
	default:
		return "Запуск проверки каждый день"
	}
}

// generateScheduleCalls генерирует вызовы функций расписания
func (pg *PluginGenerator) generateScheduleCalls(functionName, schedule string) string {
	switch {
	case strings.Contains(schedule, "6 часов"):
		return fmt.Sprintf("run_every.hour(check_%s, 6)", functionName)
	case strings.Contains(schedule, "день"):
		return fmt.Sprintf("run_every.day(check_%s)", functionName)
	case strings.Contains(schedule, "неделю"):
		return fmt.Sprintf("run_every.week(check_%s)", functionName)
	default:
		return fmt.Sprintf("run_every.day(check_%s)", functionName)
	}
}

// generateURLPattern генерирует regex паттерн для обновления URL в файле
func (pg *PluginGenerator) generateURLPattern(detected DetectedPackage) string {
	if detected.URLTemplate != "" {
		return "" // URL pattern будет сгенерирован автоматически
	}

	// Базовые паттерны для разных типов
	switch detected.GitHubRepo {
	case "telegramdesktop/tdesktop":
		return `https://github\.com/telegramdesktop/tdesktop/releases/download/[^/]+/tsetup\.[0-9.]+\.tar\.xz`
	case "obsidianmd/obsidian-releases":
		return `https://github\.com/obsidianmd/obsidian-releases/releases/download/[^/]+/Obsidian-[0-9.]+\.AppImage`
	default:
		return `https://github\.com/` + strings.ReplaceAll(detected.GitHubRepo, "/", `\/`) + `/releases/download/[^/]+/.*`
	}
}

// GenerateAllPlugins сканирует все репозитории и генерирует недостающие плагины
func (pg *PluginGenerator) GenerateAllPlugins() error {
	var totalGenerated int

	for repoName := range pg.config.Repositories {
		fmt.Printf("🔍 Сканирование репозитория %s...\n", repoName)
		
		packages, err := pg.ScanRepository(repoName)
		if err != nil {
			fmt.Printf("Warning: failed to scan repository %s: %v\n", repoName, err)
			continue
		}

		fmt.Printf("Найдено %d пакетов без плагинов\n", len(packages))

		for _, pkg := range packages {
			if pkg.GitHubRepo == "" {
				fmt.Printf("⚠️  Пропущен %s: GitHub репозиторий не определен\n", pkg.Name)
				continue
			}

			err := pg.GeneratePlugin(pkg, repoName)
			if err != nil {
				fmt.Printf("❌ Ошибка генерации плагина для %s: %v\n", pkg.Name, err)
				continue
			}
			totalGenerated++
		}
	}

	fmt.Printf("\n🎉 Всего сгенерировано %d плагинов\n", totalGenerated)
	return nil
}