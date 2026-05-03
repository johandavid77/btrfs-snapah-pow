#!/bin/bash
# Instalador de Btrfs Snapah Pow para cualquier Linux con systemd

set -e

APP_NAME="snapah-pow"
INSTALL_DIR="/opt/snapah-pow"
SERVICE_DIR="/etc/systemd/system"
USER=${SUDO_USER:-$USER}

echo "🔥 Instalando Btrfs Snapah Pow..."

# Verificar systemd
if ! command -v systemctl &> /dev/null; then
    echo "❌ systemd no encontrado. Este instalador solo funciona en sistemas con systemd."
    exit 1
fi

# Verificar BTRFS
if ! command -v btrfs &> /dev/null; then
    echo "⚠️  btrfs-progs no instalado. Instálalo primero:"
    echo "   Fedora: sudo dnf install btrfs-progs"
    echo "   Ubuntu/Debian: sudo apt install btrfs-progs"
    echo "   Arch: sudo pacman -S btrfs-progs"
    exit 1
fi

# Crear directorio
sudo mkdir -p $INSTALL_DIR/bin
sudo mkdir -p $INSTALL_DIR/data
sudo mkdir -p $INSTALL_DIR/web

# Copiar binarios
echo "📦 Copiando binarios..."
sudo cp bin/snapah-server $INSTALL_DIR/bin/
sudo cp bin/snapah-agent $INSTALL_DIR/bin/
sudo cp bin/snapah $INSTALL_DIR/bin/

# Copiar web
sudo cp -r web/* $INSTALL_DIR/web/

# Copiar config
sudo cp config.yaml $INSTALL_DIR/

# Permisos
sudo chmod +x $INSTALL_DIR/bin/*
sudo chown -R root:root $INSTALL_DIR

# Instalar servicios
echo "⚙️  Instalando servicios systemd..."
sudo cp deploy/systemd/snapah-server.service $SERVICE_DIR/
sudo cp deploy/systemd/snapah-agent.service $SERVICE_DIR/

# Recargar systemd
sudo systemctl daemon-reload

echo ""
echo "✅ Instalación completa!"
echo ""
echo "Comandos útiles:"
echo "  sudo systemctl enable --now snapah-server    # Iniciar server"
echo "  sudo systemctl enable --now snapah-agent     # Iniciar agente"
echo "  sudo systemctl status snapah-server          # Ver estado"
echo "  sudo journalctl -u snapah-server -f          # Ver logs"
echo ""
echo "Web UI: http://localhost:8081"
echo ""
