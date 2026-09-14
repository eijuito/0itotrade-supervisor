# Release Notes: 0itotrade-supervisor

Registro histórico de versões, melhorias e correções do **0itotrade-supervisor**.

---

## [v0.1.0] - 2026-09-14 (Lançamento Inicial)

Primeira versão estável do daemon supervisor em Go para o ecossistema 0itotrade.

### 🚀 Novas Funcionalidades
- **Core em Go de Alta Performance**:
  - Compilado como binário estático sem dependências externas (`CGO_ENABLED=0`).
  - Consumo de memória reduzido com meta inferior a 30MB RSS.
  - Manipulação graciosa de sinais do sistema operacional (`SIGINT`, `SIGTERM`).
- **Watchdog de Banco de Dados MySQL**:
  - Sonda de conectividade de rede TCP de alta velocidade.
  - Sonda SQL de autenticação e verificação de integridade via ping.
  - Validação contínua do SLA de latência (< 2000ms).
  - Circuit breaker integrado com backoff exponencial com jitter (2s a 60s) e limite de desarme em 5 falhas consecutivas.
- **Identificação Dinâmica por Hostname**:
  - Resolução automática do nome do nó através de `os.Hostname()`.
  - Opção de override configurável no arquivo `supervisor.yaml`.
- **Módulo de Notificações Telegram**:
  - **Relatório Diário**: Envio automático programado em horário customizável (ex: `08:00`) com resumo de integridade, latência e uptime.
  - **Alertas Imediatos**: Notificação em tempo real caso o MySQL entre em estado crítico ou seja restabelecido.
  - Comando CLI `0itotrade-supervisor test-telegram` para validação imediata de conectividade.
- **Servidor Local de Telemetria**:
  - Endpoint leve `GET http://127.0.0.1:9100/health` retornando status em formato JSON estruturado.
- **Instalador Automatizado para Ubuntu (Systemd)**:
  - Script `scripts/install-ubuntu.sh` com suporte a detecção de arquitetura (`amd64` e `arm64`), ideal para instâncias Ampere e x86_64 da Oracle Cloud Free Tier.
  - Criação de usuário sem privilégios `0itotrade` e configuração de serviço com `Restart=always` e `RestartSec=5s`.
- **CI/CD e Compilação Cruzada**:
  - Pipeline GitHub Actions para compilação e publicação automática de binários estáticos para `linux-amd64` e `linux-arm64`.

---

## 🔮 Roadmap Futuro (v0.2.0+)
- [ ] Watchdog para conectividade e instâncias do MetaTrader 5 (MT5).
- [ ] Monitoramento do QuestDB e métricas de ingestão de ticks.
- [ ] Sincronização automática com repositórios Git de outros módulos da plataforma.
- [ ] Suporte nativo a Windows Service (`.exe`).
