/*
 * ALR Updater - Automated updater bot for ALR packages
 * Copyright (C) 2025 The ALR Authors
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/caarlos0/env/v8"
	"github.com/go-git/go-git/v5"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/pflag"
	"go.elara.ws/logger"
	"go.elara.ws/logger/log"
	"gitea.plemya-x.ru/Plemya-x/ALR-updater/internal/builtins"
	"gitea.plemya-x.ru/Plemya-x/ALR-updater/internal/config"
	"go.etcd.io/bbolt"
	"go.starlark.net/starlark"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/term"
)

func init() {
	log.Logger = logger.NewPretty(os.Stderr)
}

// parseRepositoryFromPlugin парсит комментарий '# Repository: repo-name' из .star файла
func parseRepositoryFromPlugin(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	repoRegex := regexp.MustCompile(`^#\s*Repository:\s*(.+?)\s*$`)

	// Читаем первые несколько строк файла
	lineCount := 0
	for scanner.Scan() && lineCount < 10 {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		if matches := repoRegex.FindStringSubmatch(line); matches != nil {
			return strings.TrimSpace(matches[1]), nil
		}

		// Если встретили строку без комментария, прекращаем поиск
		if line != "" && !strings.HasPrefix(line, "#") {
			break
		}
	}

	return "", scanner.Err()
}

func main() {
	configPath := pflag.StringP("config", "c", "/etc/alr-updater/config.toml", "Path to config file")
	dbPath := pflag.StringP("database", "d", "/var/lib/alr-updater/db", "Path to database file")
	pluginDir := pflag.StringP("plugin-dir", "p", "/etc/alr-updater/plugins", "Path to plugin directory")
	serverAddr := pflag.StringP("address", "a", ":8080", "Webhook server address")
	genHash := pflag.BoolP("gen-hash", "g", false, "Generate a password hash for webhooks")
	useEnv := pflag.BoolP("use-env", "E", false, "Use environment variables for configuration")
	debug := pflag.BoolP("debug", "D", false, "Enable debug logging")
	runNow := pflag.BoolP("now", "n", false, "Run all plugin checks immediately on startup")
	pflag.Parse()

	if *debug {
		log.Logger.SetLevel(logger.LogLevelDebug)
	}

	if *genHash {
		fmt.Print("Password: ")
		pwd, err := term.ReadPassword(int(os.Stdin.Fd()))
		if err != nil {
			log.Fatal("Error reading password").Err(err).Send()
		}
		hash, err := bcrypt.GenerateFromPassword(pwd, bcrypt.DefaultCost)
		if err != nil {
			log.Fatal("Error hashing password").Err(err).Send()
		}
		fmt.Printf("\n%s\n", hash)
		return
	}

	db, err := bbolt.Open(*dbPath, 0o644, nil)
	if err != nil {
		log.Fatal("Error opening database").Err(err).Send()
	}

	cfg := &config.Config{}
	if *useEnv {
		err = env.Parse(cfg)
		if err != nil {
			log.Fatal("Error parsing environment variables").Err(err).Send()
		}
	} else {
		fl, err := os.Open(*configPath)
		if err != nil {
			log.Fatal("Error opening config file").Err(err).Send()
		}
		err = toml.NewDecoder(fl).Decode(cfg)
		if err != nil {
			log.Fatal("Error decoding config file").Err(err).Send()
		}
		err = fl.Close()
		if err != nil {
			log.Fatal("Error closing config file").Err(err).Send()
		}
	}

	// Создаем базовый каталог для репозиториев
	if cfg.ReposBaseDir == "" {
		cfg.ReposBaseDir = "/var/cache/alr-updater"
	}

	if _, err := os.Stat(cfg.ReposBaseDir); os.IsNotExist(err) {
		err = os.MkdirAll(cfg.ReposBaseDir, 0o755)
		if err != nil {
			log.Fatal("Error creating repositories base directory").Err(err).Send()
		}
	} else if err != nil {
		log.Fatal("Cannot stat configured repos base directory").Err(err).Send()
	}

	// Клонируем все сконфигурированные репозитории
	for repoName, repoConfig := range cfg.Repositories {
		repoDir := filepath.Join(cfg.ReposBaseDir, repoName)

		if _, err := os.Stat(repoDir); os.IsNotExist(err) {
			log.Info("Cloning repository").Str("name", repoName).Str("url", repoConfig.RepoURL).Send()

			err = os.MkdirAll(repoDir, 0o755)
			if err != nil {
				log.Fatal("Error creating repository directory").Str("repo", repoName).Err(err).Send()
			}

			_, err := git.PlainClone(repoDir, false, &git.CloneOptions{
				URL:      repoConfig.RepoURL,
				Progress: os.Stderr,
			})
			if err != nil {
				log.Fatal("Error cloning repository").Str("repo", repoName).Err(err).Send()
			}

			log.Info("Repository cloned successfully").Str("name", repoName).Send()
		} else if err != nil {
			log.Fatal("Cannot stat repository directory").Str("repo", repoName).Err(err).Send()
		} else {
			log.Info("Repository already exists").Str("name", repoName).Send()
		}
	}

	// Проверяем, что есть хотя бы один репозиторий
	if len(cfg.Repositories) == 0 {
		log.Fatal("No repositories configured. At least one repository is required.").Send()
	}

	starFiles, err := filepath.Glob(filepath.Join(*pluginDir, "*.star"))
	if err != nil {
		log.Fatal("Error finding plugin files").Err(err).Send()
	}

	if len(starFiles) == 0 {
		log.Fatal("No plugins found. At least one plugin is required.").Send()
	}

	mux := http.NewServeMux()

	for _, starFile := range starFiles {
		pluginName := filepath.Base(strings.TrimSuffix(starFile, ".star"))

		// Парсим комментарий Repository из файла плагина
		repoName, err := parseRepositoryFromPlugin(starFile)
		if err != nil {
			log.Fatal("Error parsing repository from plugin").Str("file", starFile).Err(err).Send()
		}
		if repoName == "" {
			log.Fatal("Plugin must specify repository").Str("file", starFile).Str("hint", "Add '# Repository: repo-name' comment at the top").Send()
		}

		// Проверяем, что указанный репозиторий существует в конфигурации
		if _, exists := cfg.Repositories[repoName]; !exists {
			log.Fatal("Repository not found in configuration").Str("plugin", pluginName).Str("repository", repoName).Send()
		}

		thread := &starlark.Thread{Name: pluginName}

		predeclared := starlark.StringDict{
			"REPO": starlark.String(repoName), // Передаем имя репозитория как глобальную переменную
		}
		builtins.Register(predeclared, &builtins.Options{
			Name:   pluginName,
			Config: cfg,
			DB:     db,
			Mux:    mux,
			RunNow: *runNow,
		})

		_, err = starlark.ExecFile(thread, starFile, nil, predeclared)
		if err != nil {
			log.Fatal("Error executing starlark file").Str("file", starFile).Err(err).Send()
		}

		log.Info("Initialized plugin").Str("name", pluginName).Str("repository", repoName).Send()
	}

	// Запускаем все зарегистрированные функции немедленно если установлен флаг --now
	if *runNow {
		// Получаем все функции, зарегистрированные через run_every
		registeredFns := builtins.GetRegisteredFunctions()
		
		if len(registeredFns) > 0 {
			log.Info("Running all registered plugin checks immediately").Int("functions", len(registeredFns)).Send()
			
			var wg sync.WaitGroup
			for key, fn := range registeredFns {
				parts := strings.Split(key, ":")
				pluginName := parts[0]
				
				log.Info("Executing registered function").Str("plugin", pluginName).Str("function", fn.Name()).Send()
				
				wg.Add(1)
				// Запускаем функцию в горутине для параллельного выполнения
				go func(function *starlark.Function, plugin string) {
					defer wg.Done()
					thread := &starlark.Thread{Name: plugin}
					_, err := starlark.Call(thread, function, nil, nil)
					if err != nil {
						log.Error("Error executing function").Str("plugin", plugin).Str("function", function.Name()).Err(err).Send()
					} else {
						log.Info("Function executed successfully").Str("plugin", plugin).Str("function", function.Name()).Send()
					}
				}(fn, pluginName)
			}
			
			// Ждём завершения всех функций
			log.Info("Waiting for immediate checks to complete...").Send()
			wg.Wait()
			log.Info("All checks completed, exiting").Send()
			return // Выходим после выполнения всех проверок
		} else {
			log.Warn("No functions registered with run_every, nothing to run immediately").Send()
			return
		}
	}

	log.Info("Starting HTTP server").Str("addr", *serverAddr).Send()
	http.ListenAndServe(*serverAddr, mux)
}
