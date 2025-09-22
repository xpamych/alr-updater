#!/bin/bash
set -e

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}ALR Updater Installation Script${NC}"
echo "==============================="

# Проверка прав root
if [[ $EUID -ne 0 ]]; then
   echo -e "${RED}This script must be run as root${NC}"
   exit 1
fi

# Переменные
BINARY_PATH="/usr/local/bin/alr-updater"
SERVICE_NAME="alr-updater"
SERVICE_USER="alr-updater"
SERVICE_GROUP="wheel"
CONFIG_DIR="/etc/alr-updater"
DATA_DIR="/var/lib/alr-updater"
CACHE_DIR="/var/cache/alr-updater"
PLUGIN_DIR="${CONFIG_DIR}/plugins"

# Создание пользователя и добавление в группу wheel
echo -e "${YELLOW}Creating user and adding to wheel group...${NC}"
if ! id -u ${SERVICE_USER} >/dev/null 2>&1; then
    useradd -r -s /bin/false -d /var/lib/${SERVICE_USER} -G wheel ${SERVICE_USER}
    echo -e "${GREEN}User ${SERVICE_USER} created and added to wheel group${NC}"
else
    # Добавляем существующего пользователя в группу wheel
    usermod -a -G wheel ${SERVICE_USER}
    echo -e "${GREEN}User ${SERVICE_USER} already exists, added to wheel group${NC}"
fi

# Создание директорий
echo -e "${YELLOW}Creating directories...${NC}"
mkdir -p ${CONFIG_DIR}
mkdir -p ${DATA_DIR}
mkdir -p ${CACHE_DIR}
mkdir -p ${PLUGIN_DIR}

# Установка прав доступа с setgid битом
echo -e "${YELLOW}Setting permissions with setgid...${NC}"
chown -R root:${SERVICE_GROUP} ${DATA_DIR}
chown -R root:${SERVICE_GROUP} ${CACHE_DIR}
chown -R root:${SERVICE_GROUP} ${CONFIG_DIR}
chmod 2775 ${CONFIG_DIR}
chmod 2775 ${PLUGIN_DIR}
chmod 2775 ${DATA_DIR}
chmod 2775 ${CACHE_DIR}

# Копирование бинарника
if [ -f "./alr-updater" ]; then
    echo -e "${YELLOW}Installing binary...${NC}"
    cp ./alr-updater ${BINARY_PATH}
    chmod 755 ${BINARY_PATH}
    echo -e "${GREEN}Binary installed to ${BINARY_PATH}${NC}"
else
    echo -e "${YELLOW}Binary not found in current directory, skipping binary installation${NC}"
fi

# Создание примера конфигурации, если не существует
if [ ! -f "${CONFIG_DIR}/config.toml" ]; then
    if [ -f "./alr-updater.example.toml" ]; then
        echo -e "${YELLOW}Creating example configuration...${NC}"
        cp ./alr-updater.example.toml ${CONFIG_DIR}/config.toml
        chown root:${SERVICE_GROUP} ${CONFIG_DIR}/config.toml
        chmod 640 ${CONFIG_DIR}/config.toml
        echo -e "${GREEN}Configuration created at ${CONFIG_DIR}/config.toml${NC}"
        echo -e "${YELLOW}Please edit the configuration file before starting the service${NC}"
    fi
fi

# Создание systemd service файла
echo -e "${YELLOW}Creating systemd service...${NC}"
cat > /etc/systemd/system/${SERVICE_NAME}.service << EOF
[Unit]
Description=ALR Updater Service
After=network.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
ExecStart=${BINARY_PATH}
Restart=on-failure
RestartSec=30
StandardOutput=journal
StandardError=journal
SyslogIdentifier=${SERVICE_NAME}

# Безопасность
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${DATA_DIR} ${CACHE_DIR}
ReadOnlyPaths=${CONFIG_DIR}

[Install]
WantedBy=multi-user.target
EOF

# Перезагрузка systemd
echo -e "${YELLOW}Reloading systemd...${NC}"
systemctl daemon-reload

# Включение сервиса
echo -e "${YELLOW}Enabling service...${NC}"
systemctl enable ${SERVICE_NAME}.service

echo ""
echo -e "${GREEN}Installation completed!${NC}"
echo ""
echo "Next steps:"
echo "1. Edit configuration: nano ${CONFIG_DIR}/config.toml"
echo "2. Add plugins to: ${PLUGIN_DIR}/"
echo "3. Start service: systemctl start ${SERVICE_NAME}"
echo "4. Check status: systemctl status ${SERVICE_NAME}"
echo "5. View logs: journalctl -u ${SERVICE_NAME} -f"