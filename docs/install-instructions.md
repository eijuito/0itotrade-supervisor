# Instruções de Instalação: 0itotrade-supervisor

Este documento detalha o processo de instalação, configuração e operação contínua do **0itotrade-supervisor** como um daemon `systemd` em servidores **Ubuntu Linux** (especialmente instâncias na **Oracle Cloud Infrastructure - OCI Free Tier**).

---

## 1. Instalação Automatizada em 1 Linha (Recomendada)

O script instalador oficial detecta a arquitetura do processador (`x86_64` ou `arm64`), baixa a versão estável pré-compilada das Releases do GitHub, cria o usuário de sistema `0itotrade`, estrutura os diretórios e habilita o serviço no systemd.

Execute no terminal do seu VPS Ubuntu:

```bash
# Instalação padrão (modo client/agente):
curl -sSL https://raw.githubusercontent.com/eijuito/0itotrade-supervisor/main/scripts/install-ubuntu.sh | sudo bash

# Instalação como Servidor Central / Hub:
curl -sSL https://raw.githubusercontent.com/eijuito/0itotrade-supervisor/main/scripts/install-ubuntu.sh | sudo bash -s -- --server

# Forçar/reconfigurar como Cliente:
curl -sSL https://raw.githubusercontent.com/eijuito/0itotrade-supervisor/main/scripts/install-ubuntu.sh | sudo bash -s -- --client
```

> [!TIP]
> **Atualização Automática**: Ao reexecutar o script em uma máquina onde o supervisor já está instalado, o instalador detecta a versão local contra o GitHub Releases e solicita confirmação para atualizar o binário e reiniciar o serviço systemd de forma transparente sem sobrescrever seu arquivo de configuração existente.

---

## 2. Instalação Automatizada no Windows (PowerShell)

Para instalar ou atualizar em estações de trabalho ou servidores **Windows** com registro no PATH e execução contínua no Agendador de Tarefas:

Abra o **PowerShell como Administrador** e execute:

```powershell
# Instalação padrão (modo client/agente):
irm https://raw.githubusercontent.com/eijuito/0itotrade-supervisor/main/scripts/install-windows.ps1 | iex

# Instalação como Servidor Central / Hub:
powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/eijuito/0itotrade-supervisor/main/scripts/install-windows.ps1 | iex" -Server

# Executando a partir do repositório local clonado:
powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1 -Server
```

---

## 2. Instalação Manual Passo a Passo

Caso prefira executar cada etapa manualmente:

### Passo 2.1: Criar Usuário e Pastas
```bash
# Criar usuário de sistema dedicado
sudo groupadd -r 0itotrade || true
sudo useradd -r -g 0itotrade -d /opt/0itotrade -s /usr/sbin/nologin 0itotrade || true

# Criar pastas do sistema
sudo mkdir -p /opt/0itotrade/bin /etc/0itotrade /var/log/0itotrade
```

### Passo 2.2: Baixar e Instalar o Binário
Identifique sua arquitetura (`amd64` ou `arm64`) e baixe a release:
```bash
# Exemplo para AMD64 (x86_64)
curl -sSL -o /tmp/supervisor.tar.gz https://github.com/eijuito/0itotrade-supervisor/releases/latest/download/0itotrade-supervisor-linux-amd64.tar.gz

# Extrair e mover
tar -xzf /tmp/supervisor.tar.gz -C /tmp/
sudo mv /tmp/0itotrade-supervisor /opt/0itotrade/bin/0itotrade-supervisor
sudo chmod +x /opt/0itotrade/bin/0itotrade-supervisor
sudo ln -sf /opt/0itotrade/bin/0itotrade-supervisor /usr/local/bin/0itotrade-supervisor
```

### Passo 2.3: Criar Unidade de Serviço do Systemd
Crie o arquivo `/etc/systemd/system/0itotrade-supervisor.service`:
```ini
[Unit]
Description=0itotrade-supervisor Daemon & Watchdog
After=network.target network-online.target
Wants=network-online.target

[Service]
Type=simple
User=0itotrade
Group=0itotrade
WorkingDirectory=/opt/0itotrade
ExecStart=/opt/0itotrade/bin/0itotrade-supervisor run -config /etc/0itotrade/supervisor.yaml
Restart=always
RestartSec=5s
LimitNOFILE=65536
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

---

## 3. Configuração do Sistema

Edite o arquivo de configuração criado em `/etc/0itotrade/supervisor.yaml`:

```bash
sudo nano /etc/0itotrade/supervisor.yaml
```

### Exemplo de Configuração:
```yaml
# "auto" utiliza o hostname do sistema operacional
instance_name: "auto"

server:
  host: "127.0.0.1"
  port: 9100
  auth_token: ""

mysql:
  enabled: true
  host: "10.0.0.5"        # IP interno da instância do MySQL na subnet da OCI
  port: 3306
  user: "0itotrade"
  password: "SUA_SENHA_FORTE"
  database: "0itotrade"
  check_interval: 10s     # Verificação a cada 10 segundos
  timeout: 2000ms         # Timeout máximo por sonda
  max_latency_ms: 2000    # Acima de 2000ms marca status DEGRADED

telegram:
  enabled: true
  bot_token: "SEU_BOT_TOKEN_DO_BOTFATHER"
  chat_id: "SEU_CHAT_ID"
  daily_report_time: "08:00" # Relatório diário às 08:00
  alert_on_failure: true     # Enviar alerta imediato se cair
```

Ajuste as permissões de segurança:
```bash
sudo chown -R 0itotrade:0itotrade /opt/0itotrade /etc/0itotrade /var/log/0itotrade
sudo chmod 750 /etc/0itotrade
sudo chmod 640 /etc/0itotrade/supervisor.yaml
```

---

## 4. Gerenciamento do Serviço

```bash
# Iniciar o daemon
sudo systemctl daemon-reload
sudo systemctl enable --now 0itotrade-supervisor

# Verificar se está rodando (deve exibir "active (running)")
sudo systemctl status 0itotrade-supervisor

# Reiniciar após alterar configurações
sudo systemctl restart 0itotrade-supervisor

# Parar o serviço
sudo systemctl stop 0itotrade-supervisor

# Acompanhar logs em tempo real
sudo journalctl -u 0itotrade-supervisor -f
```

---

## 5. Validação da Instalação

### 1. Testar credenciais do Telegram:
```bash
0itotrade-supervisor test-telegram
```
Se configurado corretamente, você receberá uma mensagem instantânea no seu chat do Telegram com o emoji ✅.

### 2. Checar telemetria local:
```bash
0itotrade-supervisor status
```

---

## 6. Resolução de Problemas (Troubleshooting)

### Falha de conexão com o MySQL na subnet (TCP Connection Refused / Timeout)
- Verifique a **Security List** ou **Network Security Group (NSG)** da VCN na Oracle Cloud. Certifique-se de que a porta `3306` está liberada entre as instâncias da subnet.
- No servidor MySQL, certifique-se de que a diretiva `bind-address` no arquivo `/etc/mysql/mysql.conf.d/mysqld.cnf` está configurada para `0.0.0.0` ou o IP privado da máquina, e não apenas `127.0.0.1`.
- Certifique-se de que o usuário MySQL possui permissão para conectar a partir da subnet (ex: `'0itotrade'@'10.0.0.%'`).

### Falha ao enviar para o Telegram
- Certifique-se de que o VPS possui rota de saída para a internet (Internet Gateway ativo na VCN da OCI).
- Verifique se o Bot Token foi digitado sem espaços extras e se o Chat ID é válido. Para canais/supergrupos, o Chat ID costuma começar com `-100...`.
