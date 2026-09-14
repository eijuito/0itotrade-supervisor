# Guia de Início Rápido: 0itotrade-supervisor

Este guia fornece os primeiros passos para entender, executar e operar o **0itotrade-supervisor**.

---

## 1. Visão Geral

O **0itotrade-supervisor** atua como o orquestrador e sentinela dos serviços da plataforma. Seu primeiro papel operacional é garantir que a infraestrutura de banco de dados MySQL na subnet esteja saudável e que os operadores recebam relatórios periódicos e alertas imediatos pelo Telegram.

```
                  ┌─────────────────────────────────────────┐
                  │          0itotrade-supervisor           │
                  │        (Daemon Systemd em Go)           │
                  └──────┬────────────┬─────────────┬───────┘
                         │            │             │
        ┌────────────────┘            │             └────────────────┐
        ▼                             ▼                              ▼
┌─────────────────┐         ┌───────────────────┐         ┌────────────────────┐
│ Watchdog MySQL  │         │ Notifier Telegram │         │ Servidor Telemetria│
│ - TCP Handshake │         │ - Relatório 08:00 │         │ - HTTP 127.0.0.1   │
│ - SQL Ping      │         │ - Alertas Queda   │         │   porta 9100       │
│ - SLA < 2000ms  │         │ - Alertas Volta   │         │ - /health em JSON  │
└─────────────────┘         └───────────────────┘         └────────────────────┘
```

---

## 2. Executando em Modo Interativo (Debug / Desenvolvimento)

Se você estiver desenvolvendo ou testando a configuração antes de rodar como serviço em segundo plano:

```bash
# Executar com arquivo de configuração específico
0itotrade-supervisor run -config ./configs/supervisor.example.yaml
```

**Exemplo de saída do terminal:**
```
=====================================================
 0ITOTRADE-SUPERVISOR v0.1.0
 Instância: vps-oracle-free
 Telemetria: http://127.0.0.1:9100/health
 Telegram Notifier Ativo: true
 Watchdog MySQL Subnet: 10.0.0.5:3306 (Check: 10s, Timeout: 2s)
=====================================================
[WATCHDOG] Iniciando motor de monitoramento para instância: vps-oracle-free (1 probes registradas)
[TELEMETRY] Servidor HTTP de telemetria ativo em http://127.0.0.1:9100/health
[TELEGRAM] Próximo relatório diário agendado para: 2026-09-15 08:00:00 (em 11h 25m)
```

---

## 3. Comandos Principais do CLI

### Consultar Status Atual (`status`)
Verifica a telemetria local e exibe um JSON formatado com os tempos de resposta:
```bash
0itotrade-supervisor status
```

### Testar Conexão com o Telegram (`test-telegram`)
Envia uma mensagem de teste para o bot e chat configurados para validar se os tokens estão corretos:
```bash
0itotrade-supervisor test-telegram
```

### Consultar Versão (`version`)
Exibe a versão e data de compilação do binário:
```bash
0itotrade-supervisor version
```

---

## 4. Consultando o Endpoint de Telemetria (`/health`)

Você pode integrar scripts locais, Zabbix, Prometheus ou simplesmente consultar via `curl`:

```bash
curl -s http://127.0.0.1:9100/health | jq .
```

**Exemplo de Resposta:**
```json
{
  "status": "HEALTHY",
  "instance_name": "ubuntu-oci-vps",
  "uptime_seconds": 3624.15,
  "memory_alloc_mb": 4.12,
  "memory_sys_mb": 11.45,
  "num_goroutine": 8,
  "timestamp": "2026-09-14T20:30:00Z",
  "probes": {
    "mysql_subnet": {
      "name": "mysql_subnet",
      "target": "10.0.0.5:3306",
      "status": "HEALTHY",
      "latency_ms": 12,
      "last_success": "2026-09-14T20:29:55Z",
      "consecutive_failures": 0,
      "details": {
        "threads_connected": "14"
      }
    }
  }
}
```

---

## 5. Próximos Passos
Consulte o documento de [Instruções de Instalação](./install-instructions.md) para configurar o daemon definitivo no seu VPS Ubuntu.
