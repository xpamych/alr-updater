package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/pflag"
	"gitea.plemya-x.ru/Plemya-x/ALR-updater/internal/config"
	"gitea.plemya-x.ru/Plemya-x/ALR-updater/internal/generator"
)

type AnalysisResult struct {
	PackageName     string   `json:"package_name"`
	Version         string   `json:"version"`
	GitHubRepo      string   `json:"github_repo,omitempty"`
	Sources         []string `json:"sources,omitempty"`
	Description     string   `json:"description,omitempty"`
	Homepage        string   `json:"homepage,omitempty"`
	PackageType     string   `json:"package_type"`
	HasPlugin       bool     `json:"has_plugin"`
	PluginGenerated bool     `json:"can_generate_plugin"`
}

func main() {
	configPath := pflag.StringP("config", "c", "/etc/alr-updater/config.toml", "Path to config file")
	pluginDir := pflag.StringP("plugin-dir", "p", "/etc/alr-updater/plugins", "Path to plugin directory")
	repoName := pflag.StringP("repo", "r", "alr-repo", "Repository name to analyze")
	outputFormat := pflag.StringP("format", "f", "table", "Output format: table, json")
	generateMissing := pflag.BoolP("generate", "g", false, "Generate missing plugins")
	pflag.Parse()

	// Загружаем конфигурацию
	cfg := &config.Config{}
	fl, err := os.Open(*configPath)
	if err != nil {
		fmt.Printf("Error opening config file: %v\n", err)
		os.Exit(1)
	}
	defer fl.Close()

	err = toml.NewDecoder(fl).Decode(cfg)
	if err != nil {
		fmt.Printf("Error decoding config file: %v\n", err)
		os.Exit(1)
	}

	// Создаем генератор плагинов
	gen, err := generator.NewPluginGenerator(cfg, *pluginDir)
	if err != nil {
		fmt.Printf("Error creating plugin generator: %v\n", err)
		os.Exit(1)
	}

	// Сканируем репозиторий
	packages, err := gen.ScanRepository(*repoName)
	if err != nil {
		fmt.Printf("Error scanning repository: %v\n", err)
		os.Exit(1)
	}

	// Получаем список существующих плагинов
	existingPlugins := make(map[string]bool)
	pluginFiles, err := filepath.Glob(filepath.Join(*pluginDir, "*.star"))
	if err == nil {
		for _, pluginFile := range pluginFiles {
			pluginName := strings.TrimSuffix(filepath.Base(pluginFile), ".star")
			existingPlugins[pluginName] = true
		}
	}

	// Подготавливаем результаты анализа
	var results []AnalysisResult
	var canGenerate []generator.DetectedPackage

	for _, pkg := range packages {
		result := AnalysisResult{
			PackageName:     pkg.Name,
			Version:         pkg.Version,
			GitHubRepo:      pkg.GitHubRepo,
			Sources:         pkg.Sources,
			Description:     pkg.Description,
			Homepage:        pkg.Homepage,
			PackageType:     pkg.PackageType,
			HasPlugin:       existingPlugins[pkg.Name],
			PluginGenerated: pkg.GitHubRepo != "" && pkg.PackageType == "github_release",
		}
		results = append(results, result)

		if !result.HasPlugin && result.PluginGenerated {
			canGenerate = append(canGenerate, pkg)
		}
	}

	// Добавляем пакеты с существующими плагинами
	repoPath := filepath.Join(cfg.ReposBaseDir, *repoName)
	err = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.Name() != "alr.sh" {
			return err
		}

		packageName := filepath.Base(filepath.Dir(path))
		if existingPlugins[packageName] {
			// Проверяем, есть ли уже в результатах
			found := false
			for _, r := range results {
				if r.PackageName == packageName {
					found = true
					break
				}
			}
			
			if !found {
				// Добавляем пакет с существующим плагином
				pkgInfo := analyzePackageFile(path)
				result := AnalysisResult{
					PackageName: packageName,
					Version:     pkgInfo.Version,
					Sources:     pkgInfo.Sources,
					Description: pkgInfo.Description,
					Homepage:    pkgInfo.Homepage,
					PackageType: "has_plugin",
					HasPlugin:   true,
				}
				results = append(results, result)
			}
		}
		return nil
	})

	// Выводим результаты
	if *outputFormat == "json" {
		output, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(output))
	} else {
		printTable(results, canGenerate)
	}

	// Генерируем недостающие плагины
	if *generateMissing {
		fmt.Printf("\n🚀 Generating %d missing plugins...\n", len(canGenerate))
		for _, pkg := range canGenerate {
			err := gen.GeneratePlugin(pkg, *repoName)
			if err != nil {
				fmt.Printf("❌ Error generating plugin for %s: %v\n", pkg.Name, err)
			}
		}
		fmt.Printf("✅ Plugin generation completed!\n")
	}
}

func analyzePackageFile(alrFile string) generator.PackageInfo {
	content, err := os.ReadFile(alrFile)
	if err != nil {
		return generator.PackageInfo{Name: filepath.Base(filepath.Dir(alrFile))}
	}

	info := generator.PackageInfo{
		Name: filepath.Base(filepath.Dir(alrFile)),
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "version=") {
			versionMatch := regexp.MustCompile(`version=['"]?([^'"]+)['"]?`).FindStringSubmatch(line)
			if len(versionMatch) > 1 {
				info.Version = versionMatch[1]
			}
		}

		if strings.Contains(line, "github.com") || strings.Contains(line, "https://") {
			urlMatch := regexp.MustCompile(`https://[^\s"']+`).FindAllString(line, -1)
			info.Sources = append(info.Sources, urlMatch...)
		}

		if strings.HasPrefix(line, "desc=") {
			descMatch := regexp.MustCompile(`desc=['"]?([^'"]+)['"]?`).FindStringSubmatch(line)
			if len(descMatch) > 1 {
				info.Description = descMatch[1]
			}
		}
	}

	return info
}

func printTable(results []AnalysisResult, canGenerate []generator.DetectedPackage) {
	fmt.Printf("📊 Анализ репозитория\n")
	fmt.Printf("═══════════════════════════════════════════════════════════════════════════════════\n")
	
	withPlugins := 0
	withoutPlugins := 0
	canGenerateCount := len(canGenerate)

	for _, r := range results {
		if r.HasPlugin {
			withPlugins++
		} else {
			withoutPlugins++
		}
	}

	fmt.Printf("📦 Всего пакетов: %d\n", len(results))
	fmt.Printf("✅ С плагинами: %d\n", withPlugins)
	fmt.Printf("❌ Без плагинов: %d\n", withoutPlugins)
	fmt.Printf("🤖 Можно сгенерировать: %d\n", canGenerateCount)

	if canGenerateCount > 0 {
		fmt.Printf("\n🎯 Пакеты для автогенерации:\n")
		fmt.Printf("─────────────────────────────────────────────────────────────────────────────────\n")
		for _, pkg := range canGenerate {
			fmt.Printf("📦 %-25s │ %s\n", pkg.Name, pkg.GitHubRepo)
		}
		fmt.Printf("\n💡 Запустите с флагом --generate для создания плагинов\n")
	}

	if withoutPlugins > canGenerateCount {
		fmt.Printf("\n⚠️  Пакеты требующие ручной настройки:\n")
		fmt.Printf("─────────────────────────────────────────────────────────────────────────────────\n")
		for _, r := range results {
			if !r.HasPlugin && !r.PluginGenerated {
				fmt.Printf("📦 %-25s │ %s\n", r.PackageName, r.PackageType)
			}
		}
	}
}