# 0itotrade-supervisor: Gerenciador de Sistema, Watchdog e Orquestração

O **0itotrade-supervisor** é o daemon nativo em **Go** de alta performance do ecossistema **0itotrade**, projetado para operar como **systemd daemon** (`0itotrade-supervisor.service`) no Linux (otimizado para Ubuntu em VPS na Oracle Cloud OCI Free Tier, arquiteturas `amd64` e `arm64`) e como **Windows Service** (`0itotrade-supervisor.exe`) no Windows.

Possui baixíssimo consumo de memória (< 30MB RSS), zero dependências externas de runtime e atua como o guardião operacional de conectividade, bancos de dados, serviços e orquestração.

---

## 📚 Documentação do Projeto

A documentação detalhada está organizada na pasta [`docs/`](./docs/):

| Documento | Descrição |
| :--- | :--- |
| 🚀 **[Guia de Início Rápido](./docs/get-started.md)** | Visão geral, comandos do CLI e como testar o daemon interativamente. |
| 🛠️ **[Instruções de Instalação](./docs/install-instructions.md)** | Passo a passo de instalação no Ubuntu (OCI Free Tier), configuração do systemd e troubleshooting. |
| 📋 **[Requisitos do Projeto](./docs/project-requirements.md)** | Requisitos de hardware, limites de memória (<30MB), portas de rede e premissas de segurança. |
| 📝 **[Notas de Lançamento](./docs/releasenotes.md)** | Registro de versões (Changelog v0.1.0), novas funcionalidades e roadmap. |

---

## 1. Responsabilidades e Funcionalidades Principais

1. **Watchdog e Monitoramento de Conectividade (MySQL na Subnet)**:
   - Monitoramento contínuo de latência e integridade do banco MySQL na mesma subnet.
   - Validação contínua do SLA de latência (< 2000ms).
   - Detecção automática de degradação (`DEGRADED`) ou falha crítica (`CRITICAL`).
   - Circuit Breaker nativo com backoff exponencial e jitter para evitar sobrecarga de conexões em falhas intermitentes.

2. **Identificação da Instância por Hostname do S.O.**:
   - Resolução automática do nome do nó através de `os.Hostname()` (ex: `ubuntu-oci-free`).
   - Suporte a override customizado via configuração (`instance_name: "meu-servidor"`).
   - O nome acompanha todos os logs, telemetria e relatórios.

3. **Notificações e Relatórios via Telegram**:
   - **Relatório Diário**: Disparado automaticamente todo dia no horário configurado (ex: `08:00`) contendo uptime, status do MySQL, latência e consumo de memória.
   - **Alertas Imediatos**: Envio de alertas ao detectar queda crítica do MySQL ou restauração do serviço.
   - Comando CLI para validação rápida: `0itotrade-supervisor test-telegram`.

4. **Telemetria Local HTTP**:
   - Endpoint leve `GET http://127.0.0.1:9100/health` reportando status em JSON para ferramentas de monitoramento e scripts locais.

---

## 2. Instalação Rápida no Ubuntu (Oracle Cloud Free Tier)

O instalador automatizado detecta a arquitetura (`amd64` ou `arm64`), baixa a release mais recente do GitHub, cria o usuário do sistema `0itotrade`, configura as permissões e ativa o serviço no **systemd**:

```bash
# Executar o instalador via curl
curl -sSL https://raw.githubusercontent.com/eijuito/0itotrade-supervisor/main/scripts/install-ubuntu.sh | sudo bash
```

Para instruções detalhadas ou instalação manual, consulte o guia de **[Instruções de Instalação](./docs/install-instructions.md)**.

### Configuração Pós-Instalação:
1. Edite o arquivo de configuração com o IP do MySQL e credenciais do Telegram:
   ```bash
   sudo nano /etc/0itotrade/supervisor.yaml
   ```
2. Reinicie o serviço:
   ```bash
   sudo systemctl restart 0itotrade-supervisor
   ```
3. Verifique o status e teste o Telegram:
   ```bash
   0itotrade-supervisor status
   0itotrade-supervisor test-telegram
   ```
4. Acompanhe os logs em tempo real:
   ```bash
   journalctl -u 0itotrade-supervisor -f
   ```

---

## 3. Comandos Principais do CLI

```bash
# Iniciar em modo interativo (desenvolvimento / debug)
0itotrade-supervisor run

# Consultar status atual da telemetria e probes locais
0itotrade-supervisor status

# Testar conectividade e envio de mensagem via Telegram Bot
0itotrade-supervisor test-telegram

# Exibir versão e dados de compilação
0itotrade-supervisor version
```

Consulte o **[Guia de Início Rápido](./docs/get-started.md)** para mais exemplos.

---

## 4. Exemplo de Configuração (`/etc/0itotrade/supervisor.yaml`)

```yaml
# "auto" utiliza o hostname do Sistema Operacional
instance_name: "auto"

server:
  host: "127.0.0.1"
  port: 9100
  auth_token: ""

mysql:
  enabled: true
  host: "10.0.0.5"        # IP interno do MySQL na mesma subnet
  port: 3306
  user: "0itotrade"
  password: "SUA_SENHA_AQUI"
  database: "0itotrade"
  check_interval: 10s     # Intervalo da sonda de verificação
  timeout: 2000ms         # Timeout de conexão
  max_latency_ms: 2000    # Alerta se latência for maior que 2000ms

telegram:
  enabled: true
  bot_token: "123456789:ABCdefGHIjklMNOpqrSTUvwxYZ"
  chat_id: "-100123456789"
  daily_report_time: "08:00" # Horário diário (HH:MM)
  alert_on_failure: true     # Dispara alerta imediato em falhas críticas
```

---

## 5. Compilação Local & Cross-Compilação

Para compilar manualmente os binários estáticos para Linux x86_64 e ARM64:

```bash
# Compilar binário da máquina local
make build

# Compilar binários estáticos para Linux AMD64 e ARM64
make build-linux-amd64
make build-linux-arm64

# Gerar pacotes tar.gz para release
make package-all
```
