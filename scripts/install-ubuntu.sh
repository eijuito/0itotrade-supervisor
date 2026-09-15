#!/usr/bin/env bash
# ==============================================================================
# 0ITOTRADE-SUPERVISOR: Instalador e Gerenciador Automatizado para Ubuntu
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

API_RELEASE_URL="https://api.github.com/repos/${GITHUB_REPO}/releases/latest"

echo "========================================================"
echo " 🚀 0itotrade-supervisor: Gerenciador de Instalação/Atualização"
echo "========================================================"

# 1. Checa privilégios de root
if [ "$EUID" -ne 0 ]; then
  echo "❌ Este script deve ser executado como root ou com sudo."
  echo "Uso: sudo bash $0 [--server | --client]"
  exit 1
fi

# 2. Parsing de parâmetros e detecção automática do papel (role)
EXPLICIT_MODE=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --server)
      EXPLICIT_MODE="server"
      shift
      ;;
    --client)
      EXPLICIT_MODE="client"
      shift
      ;;
    --help|-h)
      echo "Uso: sudo bash $0 [OPÇÕES]"
      echo "Opções:"
      echo "  --server   Instala/Configura no modo Servidor Hub (persistido no YAML)"
      echo "  --client   Instala/Configura no modo Agente Cliente (persistido no YAML)"
      echo "  (sem flag) Detecta o modo existente no YAML ou assume 'client' se for novo"
      exit 0
      ;;
    *)
      echo "⚠️  Parâmetro desconhecido: $1. Ignorando..."
      shift
      ;;
  esac
done

if [ -n "${EXPLICIT_MODE}" ]; then
  MODE="${EXPLICIT_MODE}"
  echo "🎯 Modo definido explicitamente via argumento: [${MODE^^}]"
elif [ -f "${CONFIG_FILE}" ] && grep -q 'role:' "${CONFIG_FILE}" 2>/dev/null; then
  DETECTED_ROLE=$(grep -E '^[[:space:]]*role:' "${CONFIG_FILE}" | head -n 1 | awk -F'"' '{print $2}')
  if [ -z "${DETECTED_ROLE}" ]; then
    DETECTED_ROLE=$(grep -E '^[[:space:]]*role:' "${CONFIG_FILE}" | head -n 1 | awk '{print $2}' | tr -d '[:space:]"')
  fi

  if [[ "${DETECTED_ROLE}" == "server" || "${DETECTED_ROLE}" == "client" ]]; then
    MODE="${DETECTED_ROLE}"
    echo "🔍 Modo detectado automaticamente da configuração existente: [${MODE^^}]"
  else
    MODE="client"
    echo "⚠️  Role inválido na configuração existente. Assumindo padrão: [CLIENT]"
  fi
else
  MODE="client"
  echo "ℹ️  Nenhuma configuração prévia encontrada. Usando modo padrão: [CLIENT]"
fi

# Se foi passado explicitamente e o arquivo já existe, sincroniza o role no supervisor.yaml
if [ -n "${EXPLICIT_MODE}" ] && [ -f "${CONFIG_FILE}" ]; then
  if grep -q 'role:' "${CONFIG_FILE}"; then
    sed -i -E "s/^[[:space:]]*role:[[:space:]]*.*/role: \"${MODE}\"/" "${CONFIG_FILE}"
  else
    sed -i "1a role: \"${MODE}\"" "${CONFIG_FILE}"
  fi
  echo "💾 Papel [${MODE^^}] sincronizado e persistido em ${CONFIG_FILE}."
fi

# 3. Detecta arquitetura do sistema operacional (x86_64 vs arm64/aarch64)
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

# 4. Verificação de Atualização Automática (se já estiver instalado)
check_and_update() {
  if [ -f "${BIN_PATH}" ]; then
    echo "🔍 Supervisor já instalado em ${BIN_PATH}. Verificando atualizações no GitHub..."
    
    if ! command -v curl >/dev/null 2>&1 || ! command -v jq >/dev/null 2>&1; then
      apt-get update -y -qq >/dev/null
      apt-get install -y -qq curl jq >/dev/null
    fi

    CURRENT_VERSION=$("${BIN_PATH}" version 2>/dev/null || "${BIN_PATH}" -v 2>/dev/null || echo "v0.0.0")
    LATEST_JSON=$(curl -sL "${API_RELEASE_URL}" || true)
    LATEST_TAG=$(echo "${LATEST_JSON}" | jq -r '.tag_name // empty' 2>/dev/null || true)

    if [ -n "${LATEST_TAG}" ] && [ "${LATEST_TAG}" != "null" ]; then
      if [ "${CURRENT_VERSION}" != "${LATEST_TAG}" ]; then
        echo ""
        echo "🔔 Nova versão disponível no repositório: ${LATEST_TAG} (Versão atual: ${CURRENT_VERSION})"
        RESP="s"
        if [ -t 0 ] || [ -c /dev/tty ]; then
          read -r -p "Deseja atualizar para a versão ${LATEST_TAG} agora? [S/n]: " RESP </dev/tty 2>/dev/null || RESP="s"
        fi
        RESP=${RESP,,}
        if [[ "$RESP" =~ ^(s|sim|y|yes|)$ ]]; then
          echo "🔄 Atualizando binário..."
          systemctl stop 0itotrade-supervisor.service || true
          
          DOWNLOAD_UP_SUCCESS=false
          TMP_UP=$(mktemp -d)
          LATEST_TAR="https://github.com/${GITHUB_REPO}/releases/latest/download/0itotrade-supervisor-linux-${ARCH}.tar.gz"
          
          if curl -sSL -f -o "${TMP_UP}/release.tar.gz" "${LATEST_TAR}"; then
            tar -xzf "${TMP_UP}/release.tar.gz" -C "${TMP_UP}"
            if [ -f "${TMP_UP}/0itotrade-supervisor" ]; then
              mv "${TMP_UP}/0itotrade-supervisor" "${BIN_PATH}"
              chmod +x "${BIN_PATH}"
              DOWNLOAD_UP_SUCCESS=true
            fi
          fi

          if [ "${DOWNLOAD_UP_SUCCESS}" = false ]; then
            RAW_UP_URL="https://github.com/${GITHUB_REPO}/releases/latest/download/0itotrade-supervisor-linux-${ARCH}"
            if curl -sSL -f -o "${BIN_PATH}" "${RAW_UP_URL}"; then
              chmod +x "${BIN_PATH}"
              DOWNLOAD_UP_SUCCESS=true
            fi
          fi

          rm -rf "${TMP_UP}"

          if [ "${DOWNLOAD_UP_SUCCESS}" = true ]; then
            systemctl start 0itotrade-supervisor.service
            echo "✅ Atualização para ${LATEST_TAG} concluída com sucesso!"
            systemctl status 0itotrade-supervisor.service --no-pager --lines=3 || true
            exit 0
          else
            echo "⚠️  Não foi possível baixar o binário pré-compilado. Prosseguindo para o fluxo padrão de verificação/compilação..."
            systemctl start 0itotrade-supervisor.service || true
          fi
        else
          echo "⏭️  Atualização ignorada pelo usuário. Prosseguindo..."
        fi
      else
        echo "✅ O 0itotrade-supervisor já está na versão mais recente (${CURRENT_VERSION})."
      fi
    fi
  fi
}

check_and_update

# 5. Cria usuário e grupo de sistema dedicados se não existirem
if ! getent group "${APP_GROUP}" >/dev/null; then
  echo "👤 Criando grupo ${APP_GROUP}..."
  groupadd -r "${APP_GROUP}"
fi

if ! id -u "${APP_USER}" >/dev/null 2>&1; then
  echo "👤 Criando usuário de sistema ${APP_USER}..."
  useradd -r -g "${APP_GROUP}" -d "${INSTALL_DIR}" -s /usr/sbin/nologin -c "0itotrade supervisor daemon" "${APP_USER}"
fi

# 6. Cria diretórios estruturais
echo "📁 Preparando diretórios em ${INSTALL_DIR}, ${CONFIG_DIR} e ${LOG_DIR}..."
mkdir -p "${BIN_DIR}" "${CONFIG_DIR}" "${LOG_DIR}"

# 7. Obtém o binário (download do GitHub Releases ou compilação local com Go)
DOWNLOAD_SUCCESS=false

# Se o binário já existe no local de destino e não foi pedida atualização, pula o download
if [ -f "${BIN_PATH}" ]; then
  DOWNLOAD_SUCCESS=true
else
  echo "📥 Baixando binário mais recente para linux-${ARCH} do GitHub Releases..."
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

  # Fallback 2: Se não houver binário nas Releases, clona o repositório e compila com Go
  if [ "${DOWNLOAD_SUCCESS}" = false ]; then
    echo "⚠️  Binário pré-compilado não encontrado nas Releases do GitHub."
    echo "📦 Iniciando modo de compilação sob demanda no Ubuntu..."
    
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
fi

if [ "${DOWNLOAD_SUCCESS}" = false ]; then
  echo "❌ Falha ao obter ou compilar o 0itotrade-supervisor."
  exit 1
fi

# Link simbólico para uso global no terminal
ln -sf "${BIN_PATH}" /usr/local/bin/0itotrade-supervisor

# 8. Cria configuração padrão caso ainda não exista
if [ ! -f "${CONFIG_FILE}" ]; then
  echo "⚙️  Criando arquivo de configuração padrão em ${CONFIG_FILE} para o modo [${MODE^^}]..."
  HOSTNAME_ACTUAL=$(hostname)

  if [ "${MODE}" = "server" ]; then
    cat <<EOF > "${CONFIG_FILE}"
# 0ITOTRADE-SUPERVISOR CONFIG (MODO SERVER / HUB)
instance_name: "${HOSTNAME_ACTUAL}"
role: "server"

server:
  host: "127.0.0.1"
  port: 9100
  auth_token: ""

mysql:
  enabled: true
  host: "10.0.0.5" # Ajuste para o IP interno do MySQL na subnet privada OCI
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
  else
    cat <<EOF > "${CONFIG_FILE}"
# 0ITOTRADE-SUPERVISOR CONFIG (MODO CLIENT / AGENT)
instance_name: "${HOSTNAME_ACTUAL}"
role: "client"

server:
  host: "127.0.0.1"
  port: 9100
  auth_token: ""

hub:
  host: "132.145.56.248" # IP público ou Tailscale da VPS Oracle
  port: 8443
  reconnect_interval: 5s

telegram:
  enabled: false
  bot_token: ""
  chat_id: ""
  daily_report_time: "08:00"
  alert_on_failure: true
EOF
  fi
else
  echo "ℹ️  Arquivo de configuração mantido em ${CONFIG_FILE}."
fi

# 9. Ajusta permissões
chown -R "${APP_USER}:${APP_GROUP}" "${INSTALL_DIR}" "${CONFIG_DIR}" "${LOG_DIR}"
chmod 755 "${CONFIG_DIR}"
chmod 644 "${CONFIG_FILE}"

# 10. Cria unidade de serviço Systemd
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

# 11. Recarrega systemd e inicia o daemon
echo "🔄 Recarregando daemons do systemd..."
systemctl daemon-reload
echo "▶️  Habilitando inicialização automática no boot..."
systemctl enable 0itotrade-supervisor.service
echo "🚀 Reiniciando 0itotrade-supervisor..."
systemctl restart 0itotrade-supervisor.service

# 12. Verificação de status
sleep 2
echo "========================================================"
echo "✅ Instalação / Atualização Concluída com Sucesso!"
echo "========================================================"
echo "• Status do Serviço:"
systemctl status 0itotrade-supervisor.service --no-pager --lines=5 || true

echo ""
echo "• Teste de Telemetria Local:"
curl -s http://127.0.0.1:9100/health || echo "(Serviço ainda iniciando...)"

echo ""
echo "========================================================"
echo "💡 Próximos Passos:"
echo "1. Edite as credenciais e parâmetros em:"
echo "   sudo nano ${CONFIG_FILE}"
echo "2. Depois de salvar, reinicie o serviço:"
echo "   sudo systemctl restart 0itotrade-supervisor"
echo "3. Acompanhe os logs em tempo real:"
echo "   journalctl -u 0itotrade-supervisor -f"
echo "========================================================"