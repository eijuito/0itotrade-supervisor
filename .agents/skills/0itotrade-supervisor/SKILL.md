---
name: 0itotrade-supervisor
description: Skill de desenvolvimento, operação e manutenção para o 0itotrade-supervisor em Go com watchdog MySQL e Telegram.
---

# 0itotrade-supervisor Skill

Guia de procedimentos, regras e fluxos de trabalho para o módulo **0itotrade-supervisor**.

## 1. Visão Geral e Responsabilidades
- **Daemon / Watchdog**: Monitoramento ativo de conectividade e processos críticos em Go.
- **Watchdog MySQL Subnet**: Probing contínuo de conectividade TCP, SQL ping e latência (< 2000ms SLA).
- **Notificações Telegram**: Relatório diário de integridade em horário pré-definido e alertas imediatos em transições de estado.
- **Identificação Dinâmica**: Atribuição automática do nome da instância pelo hostname do SO (`os.Hostname()`).
- **Ambientes**: Otimizado para Ubuntu Linux na Oracle Cloud (OCI) Free Tier (`amd64` e `arm64`) e Windows Service.

---

## 2. Comandos Operacionais

### Build & Empacotamento
```bash
# Compilar binários estáticos para Linux amd64 e arm64
make build-linux-amd64
make build-linux-arm64
make package-all
```

### Instalação no Ubuntu como Daemon
```bash
# Executar instalador automatizado
curl -sSL https://raw.githubusercontent.com/eijuito/0itotrade-supervisor/main/scripts/install-ubuntu.sh | sudo bash
```

### Comandos de Operação
```bash
# Executar manualmente / debug
0itotrade-supervisor run -config /etc/0itotrade/supervisor.yaml

# Consultar telemetria local
0itotrade-supervisor status

# Testar integração com o bot do Telegram
0itotrade-supervisor test-telegram

# Gerenciar daemon systemd
sudo systemctl status 0itotrade-supervisor
sudo systemctl restart 0itotrade-supervisor
sudo journalctl -u 0itotrade-supervisor -f
```

---

## 3. Checklist de Validação
- [x] Compilação estática com baixo footprint de memória (< 30MB RSS)
- [x] Nome da instância baseado no hostname do SO com suporte a override
- [x] Probing MySQL com medição de latência em milissegundos (< 2000ms SLA)
- [x] Circuit Breaker com backoff exponencial e jitter
- [x] Notificação e agendador diário do Telegram sem bibliotecas externas pesadas
- [x] Servidor HTTP de telemetria em `127.0.0.1:9100/health`
- [x] Instalador Ubuntu (`scripts/install-ubuntu.sh`) com serviço systemd e `Restart=always`
- [x] CI/CD via GitHub Actions (`.github/workflows/release.yml`) para `amd64` e `arm64`
