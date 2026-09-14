# Requisitos do Projeto: 0itotrade-supervisor

Este documento especifica os requisitos de hardware, software, rede e segurança necessários para a operação do **0itotrade-supervisor**.

---

## 1. Requisitos de Ambiente e Sistema Operacional

| Componente | Especificação Mínima / Recomendada |
| :--- | :--- |
| **Sistema Operacional** | Ubuntu Linux 22.04 LTS ou 24.04 LTS (suporte adicional a Debian 11/12) |
| **Arquitetura de CPU** | `x86_64` (AMD64) ou `aarch64` (ARM64 - compatível com Ampere A1 OCI) |
| **Ambiente de Nuvem** | Otimizado para instâncias VPS da **Oracle Cloud (OCI) Free Tier** |
| **Gerenciador de Inicialização** | `systemd` ativo como PID 1 |

---

## 2. Requisitos de Recursos e Performance

- **Consumo de Memória (RAM)**:
  - Meta de footprint ultra-baixo: **< 30 MB RSS** sob carga contínua.
- **Consumo de CPU**:
  - < 1% de uso médio de vCPU graças a verificações assíncronas baseadas em timers.
- **Disco**:
  - Menos de 50 MB de espaço em disco para binário, logs e arquivos de configuração em `/opt/0itotrade`.

---

## 3. Requisitos de Conectividade de Rede

- **Banco de Dados MySQL**:
  - Acesso de rede à instância do MySQL na mesma subnet ou rede local (porta padrão `3306/TCP`).
  - SLA de latência máxima configurada: **< 2000 ms**.
- **Notificações Telegram**:
  - Acesso de saída para a internet via HTTPS na porta `443/TCP` para o domínio `api.telegram.org`.
- **Telemetria Local**:
  - Bind em interface de loopback (`127.0.0.1:9100/TCP`) para consultas internas de saúde e monitoramento.

---

## 4. Requisitos de Software & Dependências

- **Execução em Produção**:
  - **Zero dependências externas de runtime**. O binário é estático e independente de bibliotecas compartilhadas C (`CGO_ENABLED=0`).
  - Utilitários padrão do Linux para o instalador: `curl`, `tar`, `systemd`.
- **Compilação a partir do Código-Fonte** (opcional):
  - Go 1.22 ou superior instalado.
  - `make` para automação dos comandos de build.

---

## 5. Requisitos de Segurança & Isolamento

- **Privilégios de Execução**:
  - O daemon deve rodar sob o usuário de sistema não privilegiado `0itotrade` (`/usr/sbin/nologin`).
- **Permissões de Arquivos**:
  - Diretório de configuração `/etc/0itotrade` protegido com permissão `750`.
  - Arquivo `/etc/0itotrade/supervisor.yaml` protegido com permissão `640` para resguardar senhas do banco e tokens de API.
- **Sinais POSIX**:
  - O daemon deve tratar adequadamente sinais de parada graciosa (`SIGTERM`, `SIGINT`) sem deixar conexões ou processos órfãos.
