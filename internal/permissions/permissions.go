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

package permissions

import (
	"os"
	"path/filepath"
)

// FixFilePermissions устанавливает права 664 для файла
// Используется для всех файлов, создаваемых программой, чтобы они были доступны
// для чтения и записи группе (wheel), даже если файл создан от пользователя alr-updater
func FixFilePermissions(filePath string) error {
	return os.Chmod(filePath, 0o664)
}

// FixDirectoryPermissions устанавливает права 2775 для директории (с setgid битом)
// Бит setgid (2000) гарантирует, что все новые файлы в директории будут принадлежать группе директории
func FixDirectoryPermissions(dirPath string) error {
	return os.Chmod(dirPath, 0o2775)
}

// FixRepoPermissions рекурсивно устанавливает права 775 для директорий и 664 для файлов
// Пропускает директорию .git, так как Git управляет правами самостоятельно
func FixRepoPermissions(path string) error {
	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Пропускаем директорию .git и её содержимое
		// Git управляет правами самостоятельно, не нужно их трогать
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		if info.IsDir() {
			// Устанавливаем права 2775 для директорий (setgid)
			return os.Chmod(filePath, 0o2775)
		} else {
			// Устанавливаем права 664 для файлов
			return os.Chmod(filePath, 0o664)
		}
	})
}