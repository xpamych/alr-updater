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

package config

type Config struct {
	ReposBaseDir string            `toml:"reposBaseDir" env:"REPOS_BASE_DIR"`
	Repositories map[string]GitRepo `toml:"repositories"`
	Webhook      Webhook           `toml:"webhook" envPrefix:"WEBHOOK_"`
}

type GitRepo struct {
	RepoURL     string      `toml:"repoURL"`
	Commit      Commit      `toml:"commit"`
	Credentials Credentials `toml:"credentials"`
}

type Credentials struct {
	Username string `toml:"username" env:"USERNAME"`
	Password string `toml:"password" env:"PASSWORD"`
}

type Commit struct {
	Name  string `toml:"name" env:"NAME"`
	Email string `toml:"email" env:"EMAIL"`
}

type Webhook struct {
	PasswordHash string `toml:"pwd_hash" env:"PASSWORD_HASH"`
}
