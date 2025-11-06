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
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// FixFilePermissions устанавливает права 664 для файла и владельца alr-updater:wheel
// Используется для всех файлов, создаваемых программой, чтобы они были доступны
// для чтения и записи группе (wheel), даже если файл создан от другого пользователя
func FixFilePermissions(filePath string) error {
	// Получаем UID и GID для alr-updater:wheel
	uid, gid, err := getAlrUpdaterIDs()
	if err != nil {
		// Если не можем получить ID (например, в dev-окружении), просто меняем права
		return os.Chmod(filePath, 0o664)
	}

	// Изменяем владельца
	if err := os.Chown(filePath, uid, gid); err != nil {
		// Если не удалось изменить владельца (недостаточно прав), игнорируем
		// но устанавливаем права
		_ = os.Chmod(filePath, 0o664)
		return err
	}

	return os.Chmod(filePath, 0o664)
}

// FixDirectoryPermissions устанавливает права 2775 для директории (с setgid битом) и владельца alr-updater:wheel
// Бит setgid (2000) гарантирует, что все новые файлы в директории будут принадлежать группе директории
func FixDirectoryPermissions(dirPath string) error {
	// Получаем UID и GID для alr-updater:wheel
	uid, gid, err := getAlrUpdaterIDs()
	if err != nil {
		// Если не можем получить ID (например, в dev-окружении), просто меняем права
		return os.Chmod(dirPath, 0o2775)
	}

	// Изменяем владельца
	if err := os.Chown(dirPath, uid, gid); err != nil {
		// Если не удалось изменить владельца (недостаточно прав), игнорируем
		// но устанавливаем права
		_ = os.Chmod(dirPath, 0o2775)
		return err
	}

	return os.Chmod(dirPath, 0o2775)
}

// getAlrUpdaterIDs возвращает UID пользователя alr-updater и GID группы wheel
func getAlrUpdaterIDs() (int, int, error) {
	// Получаем пользователя alr-updater
	u, err := user.Lookup("alr-updater")
	if err != nil {
		return -1, -1, fmt.Errorf("failed to lookup user alr-updater: %w", err)
	}

	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return -1, -1, fmt.Errorf("failed to parse UID: %w", err)
	}

	// Получаем группу wheel
	g, err := user.LookupGroup("wheel")
	if err != nil {
		return -1, -1, fmt.Errorf("failed to lookup group wheel: %w", err)
	}

	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return -1, -1, fmt.Errorf("failed to parse GID: %w", err)
	}

	return uid, gid, nil
}

// FixRepoPermissions рекурсивно устанавливает права 775 для директорий и 664 для файлов
// Включая директорию .git, чтобы обеспечить доступ к объектам Git для группы wheel
// Также изменяет владельца на alr-updater:wheel
func FixRepoPermissions(path string) error {
	// Получаем UID и GID для alr-updater:wheel
	uid, gid, err := getAlrUpdaterIDs()
	if err != nil {
		// Если не можем получить ID (например, в dev-окружении), только устанавливаем права
		return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				// Устанавливаем права 2775 для директорий (setgid)
				if err := os.Chmod(filePath, 0o2775); err != nil {
					return fmt.Errorf("failed to chmod directory %s: %w", filePath, err)
				}
			} else {
				// Устанавливаем права 664 для файлов
				if err := os.Chmod(filePath, 0o664); err != nil {
					return fmt.Errorf("failed to chmod file %s: %w", filePath, err)
				}
			}

			return nil
		})
	}

	return filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Изменяем владельца на alr-updater:wheel
		if err := os.Chown(filePath, uid, gid); err != nil {
			// Если не удалось изменить владельца (недостаточно прав), всё равно устанавливаем права
			if info.IsDir() {
				_ = os.Chmod(filePath, 0o2775)
				return fmt.Errorf("failed to chown directory %s: %w", filePath, err)
			} else {
				_ = os.Chmod(filePath, 0o664)
				return fmt.Errorf("failed to chown file %s: %w", filePath, err)
			}
		}

		if info.IsDir() {
			// Устанавливаем права 2775 для директорий (setgid)
			if err := os.Chmod(filePath, 0o2775); err != nil {
				return fmt.Errorf("failed to chmod directory %s: %w", filePath, err)
			}
		} else {
			// Устанавливаем права 664 для файлов
			if err := os.Chmod(filePath, 0o664); err != nil {
				return fmt.Errorf("failed to chmod file %s: %w", filePath, err)
			}
		}

		return nil
	})
}
