# 0itotrade-supervisor: Gerenciador de Sistema, Watchdog e Orquestração

O **0itotrade-supervisor** é o daemon/serviço unificado do ecossistema **0itotrade**, projetado para operar como **Windows Service** (`0itotrade-supervisor.exe`) no Windows e como **systemd daemon** (`0itotrade-supervisor.service`) no Linux.

Ele atua como o guardião operacional de todos os módulos, eliminando a necessidade de scripts manuais e garantindo alta disponibilidade, verificação de integridade, auto-atualização e reporte de telemetria local e remoto.

---

## 1. Responsabilidades Principais

1. **Watchdog e Auto-Recuperação (Health & Keepalive)**:
   - Monitoramento ativo de processos críticos: `0itotrade-mt5connector`, `0itotrade-node-agent`, `0itotrade-controller`, instâncias MT5 e `QuestDB`.
   - Reinicialização automática graciosa ou forçada em caso de falha, travamento de socket ou esgotamento de memória.
   - Detecção de processos órfãos ou zumbis (`zombie process reaper`).

2. **Gerenciamento de Ambientes e Caminhos Universais**:
   - Resolve automaticamente caminhos e variáveis de ambiente conforme o SO:
     - **Windows Dev**: `C:\0itotrade\`
     - **Linux Dev**: `/home/<usuario>/0itotrade/`
     - **Linux Produção**: `/opt/0itotrade/`
   - Checagem e validação dos pré-requisitos de runtime (Python 3.11+, Java 21 JDK, MetaTrader 5, QuestDB).

3. **Verificação de Dependências e Atualizações Contínuas**:
   - Sincronização e validação de versão dos módulos e contratos (`0itotrade-proto`).
   - Checagem automática de novas releases/commits nos repositórios remotos oficiais (`eijuito/0itotrade-*`).
   - Execução de migrações seguras com *dry-run* e capacidade de *rollback*.

4. **Telemetria de Saúde Local e Remota**:
   - **Local**: Endpoint HTTP leve (`localhost:9100/health`) e socket IPC expondo status, consumo de CPU/RAM e métricas de cada serviço filho.
   - **Remoto**: Envio periódico de heartbeat criptografado para o `0itotrade-dashboard` e alertas via Webhook/gRPC em caso de anomalias operacionais.

---

## 2. Arquitetura do Supervisor

```
                  ┌─────────────────────────────────────┐
                  │       0itotrade-supervisor          │
                  │   (Windows Service / systemd)       │
                  └──────────────────┬──────────────────┘
                                     │
         ┌───────────────────────────┼───────────────────────────┐
         ▼                           ▼                           ▼
┌──────────────────┐       ┌──────────────────┐       ┌──────────────────┐
│  Watchdog Loop   │       │ Dependency Check │       │ Telemetry Server │
│ - MT5 Connector  │       │ - Git sync check │       │ - HTTP /health   │
│ - Node Agent     │       │ - Python venvs   │       │ - IPC Socket     │
│ - QuestDB Alive  │       │ - Java JDK 21    │       │ - Dashboard Push │
└──────────────────┘       └──────────────────┘       └──────────────────┘
```

---

## 3. Comandos Principais do CLI

```bash
# Iniciar em modo interativo (desenvolvimento / debug)
0itotrade-supervisor run

# Checar integridade de todos os serviços e dependências instaladas
0itotrade-supervisor status

# Verificar e aplicar atualizações de módulos
0itotrade-supervisor update --check

# Instalar como serviço de sistema operacional
0itotrade-supervisor service install
0itotrade-supervisor service start
```
