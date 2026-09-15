#!/usr/bin/env bash
# ==============================================================================
# 0ITOTRADE-SUPERVISOR: Instalador Automatizado para Ubuntu (Systemd Daemon)
# Otimizado para VPS na Oracle Cloud (OCI) Free Tier (suporta amd64 e arm64)
# ==============================================================================

set -euo pipefail

GITHUB_REPO="eijuito/0itotrade-supervisor"
INSTALL_DIR="/opt/0itotrade"
BIN_DIR="${INSTALL_DIR}/bin"
BIN_PATH="${BIN_DIR}/0itotrade-supervisor"
CONFIG_DIR="/etc/0itotrade"
CONFIG_FILE="${CONFIG_DIR}/supervisor.yaml"
LOG_DIR="/var/log/0itotrade"
SYSTEMD_SERVICE="/etc/systemd/system/0itotrade-supervisor.service"
APP_USER="0itotrade"
APP_GROUP="0itotrade"

echo "========================================================"
echo " 🚀 Instalando 0itotrade-supervisor como Daemon Systemd"
echo "========================================================"

# 1. Checa privilégios de root
if [ "$EUID" -ne 0 ]; then
  echo "❌ Este script deve ser executado como root ou com sudo."
  echo "Uso: sudo bash $0"
  exit 1
fi

# 2. Detecta arquitetura do sistema operacional (x86_64 vs arm64/aarch64)
ARCH_RAW=$(uname -m)
case "${ARCH_RAW}" in
  x86_64)
    ARCH="amd64"
    ;;
  aarch64|arm64)
    ARCH="arm64"
    ;;
  *)
    echo "❌ Arquitetura não suportada: ${ARCH_RAW}"
    exit 1
    ;;
esac
echo "🔍 Arquitetura detectada: ${ARCH_RAW} -> ${ARCH}"

# 3. Cria usuário e grupo de sistema dedicados se não existirem
if ! getent group "${APP_GROUP}" >/dev/null; then
  echo "👤 Criando grupo ${APP_GROUP}..."
  groupadd -r "${APP_GROUP}"
fi

if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  echo "👤 Criando usuário de sistema ${APP_USER}..."
  useradd -r -g "${APP_GROUP}" -d "${INSTALL_DIR}" -s /usr/sbin/nologin -c "0itotrade supervisor daemon" "${APP_USER}"
fi

# 4. Cria diretórios estruturais
echo "📁 Preparando diretórios em ${INSTALL_DIR}, ${CONFIG_DIR} e ${LOG_DIR}..."
mkdir -p "${BIN_DIR}" "${CONFIG_DIR}" "${LOG_DIR}"

# 5. Obtém o binário (download do GitHub Releases ou compilação local se disponível)
echo "📥 Baixando binário mais recente para linux-${ARCH} do GitHub Releases..."
DOWNLOAD_SUCCESS=false

# Tenta baixar o binário pré-compilado das Releases
LATEST_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/0itotrade-supervisor-linux-${ARCH}.tar.gz"
TMP_DIR=$(mktemp -d)
trap 'rm -rf "${TMP_DIR}"' EXIT

if curl -sSL -f -o "${TMP_DIR}/release.tar.gz" "${LATEST_URL}"; then
  echo "📦 Extraindo pacote compactado..."
  tar -xzf "${TMP_DIR}/release.tar.gz" -C "${TMP_DIR}"
  if [ -f "${TMP_DIR}/0itotrade-supervisor" ]; then
    mv "${TMP_DIR}/0itotrade-supervisor" "${BIN_PATH}"
    chmod +x "${BIN_PATH}"
    DOWNLOAD_SUCCESS=true
  fi
fi

# Fallback 1: download direto do binário sem tar.gz
if [ "${DOWNLOAD_SUCCESS}" = false ]; then
  RAW_BIN_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/0itotrade-supervisor-linux-${ARCH}"
  if curl -sSL -f -o "${BIN_PATH}" "${RAW_BIN_URL}"; then
    chmod +x "${BIN_PATH}"
    DOWNLOAD_SUCCESS=true
  fi
fi

# Fallback 2: Se não houver binário nas Releases, clona o código e compila automaticamente
if [ "${DOWNLOAD_SUCCESS}" = false ]; then
  echo "⚠️  Binário pré-compilado não encontrado nas Releases do GitHub."
  echo "📦 Iniciando modo de compilação sob demanda no Ubuntu..."
  
  # Instala git e golang se não estiverem presentes
  if ! command -v git >/dev/null 2>&1 || ! command -v go >/dev/null 2>&1; then
    echo "⚙️  Instalando pré-requisitos de compilação (git, golang-go)..."
    export DEBIAN_FRONTEND=noninteractive
    apt-get update -y
    apt-get install -y git golang-go
  fi

  CLONE_DIR="${TMP_DIR}/repo"
  echo "📥 Clonando código fonte de https://github.com/${GITHUB_REPO}.git..."
  git clone --depth 1 "https://github.com/${GITHUB_REPO}.git" "${CLONE_DIR}"

  echo "🔨 Compilando binário estático para linux-${ARCH}..."
  (
    cd "${CLONE_DIR}"
    rm -f go.sum
    go mod tidy
    CGO_ENABLED=0 go build -ldflags="-s -w" -o "${BIN_PATH}" ./cmd/supervisor
  )

  if [ -f "${BIN_PATH}" ]; then
    chmod +x "${BIN_PATH}"
    DOWNLOAD_SUCCESS=true
    echo "✅ Binário compilado e instalado com sucesso em ${BIN_PATH}!"
  fi
fi

if [ "${DOWNLOAD_SUCCESS}" = false ]; then
  echo "❌ Falha ao obter ou compilar o 0itotrade-supervisor."
  exit 1
fi

# Link simbólico para uso global no terminal
ln -sf "${BIN_PATH}" /usr/local/bin/0itotrade-supervisor

# 6. Cria configuração padrão caso ainda não exista
if [ ! -f "${CONFIG_FILE}" ]; then
  echo "⚙️  Criando arquivo de configuração padrão em ${CONFIG_FILE}..."
  HOSTNAME_ACTUAL=$(hostname)
  cat <<EOF > "${CONFIG_FILE}"
# 0ITOTRADE-SUPERVISOR CONFIG
instance_name: "${HOSTNAME_ACTUAL}"

server:
  host: "127.0.0.1"
  port: 9100
  auth_token: ""

mysql:
  enabled: true
  host: "10.0.0.5" # Ajuste para o IP interno do MySQL na subnet
  port: 3306
  user: "0itotrade"
  password: "SEU_PASSWORD_AQUI"
  database: "0itotrade"
  check_interval: 10s
  timeout: 2000ms
  max_latency_ms: 2000

telegram:
  enabled: false
  bot_token: ""
  chat_id: ""
  daily_report_time: "08:00"
  alert_on_failure: true
EOF
fi

# 7. Ajusta permissões
chown -R "${APP_USER}:${APP_GROUP}" "${INSTALL_DIR}" "${CONFIG_DIR}" "${LOG_DIR}"
chmod 755 "${CONFIG_DIR}"
chmod 644 "${CONFIG_FILE}"

# 8. Cria unidade de serviço Systemd
echo "📄 Gerando serviço systemd em ${SYSTEMD_SERVICE}..."
cat <<EOF > "${SYSTEMD_SERVICE}"
[Unit]
Description=0itotrade-supervisor Daemon & Watchdog
Documentation=https://github.com/${GITHUB_REPO}
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${APP_USER}
Group=${APP_GROUP}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_PATH} run -config ${CONFIG_FILE}
Restart=always
RestartSec=5s
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# 9. Recarrega systemd e inicia o daemon
echo "🔄 Recarregando daemons do systemd..."
systemctl daemon-reload
echo "▶️  Habilitando inicialização automática no boot..."
systemctl enable 0itotrade-supervisor.service
echo "🚀 Iniciando 0itotrade-supervisor..."
systemctl restart 0itotrade-supervisor.service

# 10. Verificação de status
sleep 2
echo "========================================================"
echo "✅ Instalação Concluída com Sucesso!"
echo "========================================================"
echo "• Status do Serviço:"
systemctl status 0itotrade-supervisor.service --no-pager --lines=5 || true

echo ""
echo "• Teste de Telemetria Local:"
curl -s http://127.0.0.1:9100/health || echo "(Serviço ainda iniciando...)"

echo ""
echo "========================================================"
echo "💡 Próximos Passos:"
echo "1. Edite as credenciais do MySQL e Telegram em:"
echo "   sudo nano ${CONFIG_FILE}"
echo "2. Depois de salvar, reinicie o serviço:"
echo "   sudo systemctl restart 0itotrade-supervisor"
echo "3. Teste o envio pelo Telegram com:"
echo "   0itotrade-supervisor test-telegram"
echo "4. Acompanhe os logs em tempo real:"
echo "   journalctl -u 0itotrade-supervisor -f"
echo "========================================================"
