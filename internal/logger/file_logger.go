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

package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type RotatingFileWriter struct {
	mu          sync.Mutex
	file        *os.File
	filename    string
	maxSize     int64
	currentSize int64
}

func NewRotatingFileWriter(filename string, maxSize int64) (*RotatingFileWriter, error) {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	rfw := &RotatingFileWriter{
		filename: filename,
		maxSize:  maxSize,
	}

	if err := rfw.openFile(); err != nil {
		return nil, err
	}

	return rfw, nil
}

func (rfw *RotatingFileWriter) openFile() error {
	file, err := os.OpenFile(rfw.filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		file.Close()
		return fmt.Errorf("failed to stat log file: %w", err)
	}

	rfw.file = file
	rfw.currentSize = info.Size()
	return nil
}

func (rfw *RotatingFileWriter) rotate() error {
	if rfw.file != nil {
		rfw.file.Close()
	}

	// Переименовываем текущий файл с временной меткой
	backupName := fmt.Sprintf("%s.%s", rfw.filename, time.Now().Format("20060102-150405"))
	if err := os.Rename(rfw.filename, backupName); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to rotate log file: %w", err)
	}

	// Создаем новый файл
	if err := rfw.openFile(); err != nil {
		return err
	}

	// Удаляем старые резервные копии (оставляем только последние 5)
	pattern := rfw.filename + ".*"
	matches, _ := filepath.Glob(pattern)
	if len(matches) > 5 {
		// Сортировка не требуется, так как имена файлов содержат временную метку
		for i := 0; i < len(matches)-5; i++ {
			os.Remove(matches[i])
		}
	}

	return nil
}

func (rfw *RotatingFileWriter) Write(p []byte) (n int, err error) {
	rfw.mu.Lock()
	defer rfw.mu.Unlock()

	// Проверяем, нужна ли ротация
	if rfw.currentSize+int64(len(p)) > rfw.maxSize {
		if err := rfw.rotate(); err != nil {
			return 0, err
		}
	}

	n, err = rfw.file.Write(p)
	if err != nil {
		return n, err
	}

	rfw.currentSize += int64(n)
	return n, nil
}

func (rfw *RotatingFileWriter) Close() error {
	rfw.mu.Lock()
	defer rfw.mu.Unlock()

	if rfw.file != nil {
		return rfw.file.Close()
	}
	return nil
}

// MultiWriter объединяет вывод в несколько writers
type MultiWriter struct {
	writers []io.Writer
}

func NewMultiWriter(writers ...io.Writer) *MultiWriter {
	return &MultiWriter{writers: writers}
}

func (mw *MultiWriter) Write(p []byte) (n int, err error) {
	for _, w := range mw.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}