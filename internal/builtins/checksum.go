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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"
)

func checksumModule() *starlarkstruct.Module {
	return &starlarkstruct.Module{
		Name: "checksum",
		Members: starlark.StringDict{
			"calculate_sha256": starlark.NewBuiltin("checksum.calculate_sha256", calculateSHA256),
		},
	}
}

func calculateSHA256(thread *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var url string
	err := starlark.UnpackArgs("checksum.calculate_sha256", args, kwargs, "url", &url)
	if err != nil {
		return nil, err
	}

	// Создаем HTTP клиент с таймаутом
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	// Загружаем файл
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	// Вычисляем SHA256
	hasher := sha256.New()
	_, err = io.Copy(hasher, resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate hash: %w", err)
	}

	// Возвращаем хеш в виде hex строки
	hashSum := hex.EncodeToString(hasher.Sum(nil))
	return starlark.String(hashSum), nil
}