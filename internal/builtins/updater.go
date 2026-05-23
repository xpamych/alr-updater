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

package builtins

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"go.elara.ws/logger/log"
	"go.elara.ws/vercmp"
	"gitea.plemya-x.ru/Plemya-x/ALR-updater/internal/config"
	"gitea.plemya-x.ru/Plemya-x/ALR-updater/internal/permissions"
	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func updaterModule(cfg *config.Config) *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "updater",
		Members: starlark.StringDict{
			"repos_base_dir":     starlark.String(cfg.ReposBaseDir),
			"pull":               updaterPull(cfg),
			"push_changes":       updaterPushChanges(cfg),
			"get_package_file":   getPackageFile(cfg),
			"write_package_file": writePackageFile(cfg),
			"update_checksums":   updateChecksums(cfg),
		},
	}
}

// repoMtxMap provides a mutex for each repository to avoid blocking
// operations on one repo from affecting another
var repoMtxMap = make(map[string]*sync.Mutex)
var repoMtxMapMtx = &sync.Mutex{} // protects the map itself

func getRepoMutex(repoName string) *sync.Mutex {
	repoMtxMapMtx.Lock()
	defer repoMtxMapMtx.Unlock()
	
	mtx, exists := repoMtxMap[repoName]
	if !exists {
		mtx = &sync.Mutex{}
		repoMtxMap[repoName] = mtx
	}
	
	return mtx
}

func updaterPull(cfg *config.Config) *starlark.Builtin {
	return starlark.NewBuiltin("updater.pull", func(thread *starlark.Thread, fn *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var repoName string
		err := starlark.UnpackArgs("updater.pull", args, kwargs, "repo", &repoName)
		if err != nil {
			return nil, err
		}

		repoConfig, exists := cfg.Repositories[repoName]
		if !exists {
			return nil, fmt.Errorf("repository '%s' not found in configuration", repoName)
		}

		mtx := getRepoMutex(repoName)
		mtx.Lock()
		defer mtx.Unlock()

		repoDir := filepath.Join(cfg.ReposBaseDir, repoName)
		repo, err := git.PlainOpen(repoDir)
		if err != nil {
			return nil, err
		}

		w, err := repo.Worktree()
		if err != nil {
			return nil, err
		}

		err = w.Pull(&git.PullOptions{Progress: os.Stderr})
		if err != git.NoErrAlreadyUpToDate && err != nil {
			return nil, err
		}

		// Исправляем права доступа после git pull
		err = permissions.FixRepoPermissions(repoDir)
		if err != nil {
			log.Warn("Failed to fix repository permissions after pull").Str("repo", repoName).Err(err).Send()
		}

		_ = repoConfig // Избегаем неиспользованной переменной
		return starlark.None, nil
	})
}


func updaterPushChanges(cfg *config.Config) *starlark.Builtin {
	return starlark.NewBuiltin("updater.push_changes", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var repoName, msg string
		err := starlark.UnpackArgs("updater.push_changes", args, kwargs, "repo", &repoName, "msg", &msg)
		if err != nil {
			return nil, err
		}

		repoConfig, exists := cfg.Repositories[repoName]
		if !exists {
			return nil, fmt.Errorf("repository '%s' not found in configuration", repoName)
		}

		mtx := getRepoMutex(repoName)
		mtx.Lock()
		defer mtx.Unlock()

		repoDir := filepath.Join(cfg.ReposBaseDir, repoName)
		repo, err := git.PlainOpen(repoDir)
		if err != nil {
			return nil, err
		}

		w, err := repo.Worktree()
		if err != nil {
			return nil, err
		}

		// Сначала pull — всегда, даже если локально нет изменений
		err = w.Pull(&git.PullOptions{Progress: os.Stderr})
		if err != git.NoErrAlreadyUpToDate && err != nil {
			return nil, err
		}

		status, err := w.Status()
		if err != nil {
			return nil, err
		}

		if status.IsClean() {
			return starlark.None, nil
		}

		_, err = w.Add(".")
		if err != nil {
			return nil, err
		}

		sig := &object.Signature{
			Name:  repoConfig.Commit.Name,
			Email: repoConfig.Commit.Email,
			When:  time.Now(),
		}

		h, err := w.Commit(msg, &git.CommitOptions{
			Author:    sig,
			Committer: sig,
		})
		if err != nil {
			return nil, err
		}

		log.Debug("Created new commit").Stringer("hash", h).Stringer("pos", thread.CallFrame(1).Pos).Send()

		err = repo.Push(&git.PushOptions{
			Progress: os.Stderr,
			Auth: &http.BasicAuth{
				Username: repoConfig.Credentials.Username,
				Password: repoConfig.Credentials.Password,
			},
		})
		if err != nil {
			return nil, err
		}

		log.Debug("Successfully pushed to repo").Str("repo", repoName).Stringer("pos", thread.CallFrame(1).Pos).Send()

		return starlark.None, nil
	})
}

func getPackageFile(cfg *config.Config) *starlark.Builtin {
	return starlark.NewBuiltin("updater.get_package_file", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var repoName, pkg, filename string
		err := starlark.UnpackArgs("updater.get_package_file", args, kwargs, "repo", &repoName, "pkg", &pkg, "filename", &filename)
		if err != nil {
			return nil, err
		}

		_, exists := cfg.Repositories[repoName]
		if !exists {
			return nil, fmt.Errorf("repository '%s' not found in configuration", repoName)
		}

		// Для этой функции мы не обязательно нуждаемся в мьютексе Git,
		// так как это только чтение файлов, а не Git операции
		// Но для консистентности и безопасности будем использовать мьютекс
		mtx := getRepoMutex(repoName)
		mtx.Lock()
		defer mtx.Unlock()

		repoDir := filepath.Join(cfg.ReposBaseDir, repoName)
		path := filepath.Join(repoDir, pkg, filename)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		log.Debug("Got package file").Str("repo", repoName).Str("package", pkg).Str("filename", filename).Stringer("pos", thread.CallFrame(1).Pos).Send()
		return starlark.String(data), nil
	})
}

func writePackageFile(cfg *config.Config) *starlark.Builtin {
	return starlark.NewBuiltin("updater.write_package_file", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var repoName, pkg, filename, content string
		err := starlark.UnpackArgs("updater.write_package_file", args, kwargs, "repo", &repoName, "pkg", &pkg, "filename", &filename, "content", &content)
		if err != nil {
			return nil, err
		}

		_, exists := cfg.Repositories[repoName]
		if !exists {
			return nil, fmt.Errorf("repository '%s' not found in configuration", repoName)
		}

		// Используем мьютекс для согласованности с другими операциями
		mtx := getRepoMutex(repoName)
		mtx.Lock()
		defer mtx.Unlock()

		repoDir := filepath.Join(cfg.ReposBaseDir, repoName)
		path := filepath.Join(repoDir, pkg, filename)
		
		// Читаем старый файл для сравнения версий
		var oldContent string
		if oldData, err := os.ReadFile(path); err == nil {
			oldContent = string(oldData)
		}
		
		// Автоматически сбрасываем release='1' при изменении версии
		finalContent := autoResetRelease(oldContent, content)
		
		fl, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		defer fl.Close()

		_, err = io.Copy(fl, strings.NewReader(finalContent))
		if err != nil {
			return nil, err
		}

		// Восстанавливаем права на файл
		if err := permissions.FixFilePermissions(path); err != nil {
			log.Warn("Failed to fix file permissions").Str("path", path).Err(err).Send()
		}

		log.Debug("Wrote package file").Str("repo", repoName).Str("package", pkg).Str("filename", filename).Stringer("pos", thread.CallFrame(1).Pos).Send()
		return starlark.None, nil
	})
}

// autoResetRelease автоматически сбрасывает release='1' при увеличении версии
func autoResetRelease(oldContent, newContent string) string {
	// Извлекаем версии из старого и нового содержимого
	versionRegex := regexp.MustCompile(`version='([^']+)'`)
	
	oldVersionMatch := versionRegex.FindStringSubmatch(oldContent)
	newVersionMatch := regexp.MustCompile(`version='([^']+)'`).FindStringSubmatch(newContent)
	
	// Если версии нет в одном из файлов, возвращаем новое содержимое как есть
	if len(oldVersionMatch) < 2 || len(newVersionMatch) < 2 {
		return newContent
	}
	
	oldVersion := oldVersionMatch[1]
	newVersion := newVersionMatch[1]
	
	// Сбрасываем release только если версия увеличилась
	if vercmp.Compare(newVersion, oldVersion) > 0 {
		releaseRegex := regexp.MustCompile(`release='[^']+'`)
		if releaseRegex.MatchString(newContent) {
			return releaseRegex.ReplaceAllString(newContent, "release='1'")
		}
	}
	
	return newContent
}

// updateChecksumsInContent обновляет значения checksums в содержимом файла
func updateChecksumsInContent(content string, checksums []string) string {
	// Паттерн для поиска массива checksums
	checksumsRegex := regexp.MustCompile(`checksums=\((.*?)\)`)
	
	// Формируем новый массив checksums
	var newChecksumsArray string
	if len(checksums) == 1 && checksums[0] != "" {
		// Если одна хеш-сумма, форматируем как ('hash')
		newChecksumsArray = fmt.Sprintf("('%s')", checksums[0])
	} else if len(checksums) > 1 {
		// Если несколько хеш-сумм, форматируем как ('hash1' 'hash2' ...)
		quotedChecksums := make([]string, len(checksums))
		for i, cs := range checksums {
			quotedChecksums[i] = fmt.Sprintf("'%s'", cs)
		}
		newChecksumsArray = "(" + strings.Join(quotedChecksums, " ") + ")"
	} else {
		// Если нет хеш-сумм, оставляем SKIP
		newChecksumsArray = "('SKIP')"
	}
	
	// Заменяем старый массив checksums на новый
	newContent := checksumsRegex.ReplaceAllString(content, "checksums="+newChecksumsArray)
	
	return newContent
}

func updateChecksums(cfg *config.Config) *starlark.Builtin {
	return starlark.NewBuiltin("updater.update_checksums", func(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
		var repoName, pkg, filename, content string
		var checksums *starlark.List
		err := starlark.UnpackArgs("updater.update_checksums", args, kwargs, 
			"repo", &repoName,
			"pkg", &pkg, 
			"filename", &filename, 
			"content", &content,
			"checksums", &checksums)
		if err != nil {
			return nil, err
		}

		_, exists := cfg.Repositories[repoName]
		if !exists {
			return nil, fmt.Errorf("repository '%s' not found in configuration", repoName)
		}

		// Конвертируем starlark.List в []string
		checksumStrings := make([]string, 0, checksums.Len())
		iter := checksums.Iterate()
		defer iter.Done()
		var val starlark.Value
		for iter.Next(&val) {
			if s, ok := val.(starlark.String); ok {
				checksumStrings = append(checksumStrings, string(s))
			}
		}

		// Обновляем checksums в содержимом
		updatedContent := updateChecksumsInContent(content, checksumStrings)
		
		// Автоматически сбрасываем release='1' при изменении версии
		mtx := getRepoMutex(repoName)
		mtx.Lock()
		defer mtx.Unlock()
		
		repoDir := filepath.Join(cfg.ReposBaseDir, repoName)
		path := filepath.Join(repoDir, pkg, filename)
		
		// Читаем старый файл для сравнения версий
		var oldContent string
		if oldData, err := os.ReadFile(path); err == nil {
			oldContent = string(oldData)
		}
		
		// Применяем autoResetRelease
		finalContent := autoResetRelease(oldContent, updatedContent)
		
		// Записываем файл
		fl, err := os.Create(path)
		if err != nil {
			return nil, err
		}
		defer fl.Close()

		_, err = io.Copy(fl, strings.NewReader(finalContent))
		if err != nil {
			return nil, err
		}

		// Восстанавливаем права на файл
		if err := permissions.FixFilePermissions(path); err != nil {
			log.Warn("Failed to fix file permissions").Str("path", path).Err(err).Send()
		}

		log.Debug("Updated package file with checksums").
			Str("repo", repoName).
			Str("package", pkg).
			Str("filename", filename).
			Int("checksums_count", len(checksumStrings)).
			Stringer("pos", thread.CallFrame(1).Pos).Send()

		return starlark.None, nil
	})
}
