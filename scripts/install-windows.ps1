<#
.SYNOPSIS
    0itotrade-supervisor: Instalador e Gerenciador Automatizado para Windows.
.DESCRIPTION
    Script equivalente ao install-ubuntu.sh para ambiente Windows.
    Suporta instalacao como Servidor (Hub) ou Cliente (Agente), auto-atualizacao do binario,
    registro no PATH do sistema e inicializacao continua via Agendador de Tarefas do Windows (Task Scheduler).
.PARAMETER Server
    Instala ou configura o supervisor no modo Servidor Hub.
.PARAMETER Client
    Instala ou configura o supervisor no modo Agente Cliente.
.EXAMPLE
    irm https://raw.githubusercontent.com/eijuito/0itotrade-supervisor/main/scripts/install-windows.ps1 | iex
.EXAMPLE
    powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1 -Server
#>

[CmdletBinding()]
param(
    [switch]$Server,
    [switch]$Client,
    [switch]$Help
)

$ErrorActionPreference = "Stop"

if ($Help) {
    Write-Host "Uso: powershell -ExecutionPolicy Bypass -File .\install-windows.ps1 [OPCOES]" -ForegroundColor Cyan
    Write-Host "Opcoes:"
    Write-Host "  -Server   Instala/Configura no modo Servidor Hub (persistido no YAML)"
    Write-Host "  -Client   Instala/Configura no modo Agente Cliente (persistido no YAML)"
    Write-Host "  (sem flag) Detecta o modo existente no YAML ou assume 'client' se for novo"
    exit 0
}

# ==============================================================================
# Configuracoes e Caminhos Globais
# ==============================================================================
$GITHUB_REPO = "eijuito/0itotrade-supervisor"
$INSTALL_DIR = "$env:ProgramFiles\0itotrade\supervisor"
$BIN_DIR = "$INSTALL_DIR\bin"
$BIN_PATH = "$BIN_DIR\0itotrade-supervisor.exe"
$CONFIG_DIR = "$env:ProgramData\0itotrade"
$CONFIG_FILE = "$CONFIG_DIR\supervisor.yaml"
$LOG_DIR = "$CONFIG_DIR\logs"
$TASK_NAME = "0itotrade-supervisor"
$API_RELEASE_URL = "https://api.github.com/repos/$GITHUB_REPO/releases/latest"

Write-Host "========================================================" -ForegroundColor Cyan
Write-Host " [0itotrade-supervisor] Gerenciador de Instalacao / Atualizacao Windows" -ForegroundColor Cyan
Write-Host "========================================================" -ForegroundColor Cyan

# 1. Checa privilegios de Administrador
$isAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $isAdmin) {
    Write-Host "[ERRO] Este script deve ser executado como Administrador." -ForegroundColor Red
    Write-Host "Abra o PowerShell como Administrador e tente novamente." -ForegroundColor Yellow
    exit 1
}

# 2. Parsing e Definicao do Papel (Role)
$EXPLICIT_MODE = ""
if ($Server) {
    $EXPLICIT_MODE = "server"
} elseif ($Client) {
    $EXPLICIT_MODE = "client"
}

$MODE = "client"
if ($EXPLICIT_MODE -ne "") {
    $MODE = $EXPLICIT_MODE
    Write-Host "[MODO] Modo definido explicitamente via argumento: [$($MODE.ToUpper())]" -ForegroundColor Green
} elseif (Test-Path $CONFIG_FILE) {
    $roleLine = Get-Content $CONFIG_FILE | Where-Object { $_ -match '^\s*role:' } | Select-Object -First 1
    if ($roleLine -match 'server') {
        $MODE = "server"
        Write-Host "[MODO] Modo detectado automaticamente da configuracao existente: [SERVER]" -ForegroundColor Green
    } elseif ($roleLine -match 'client') {
        $MODE = "client"
        Write-Host "[MODO] Modo detectado automaticamente da configuracao existente: [CLIENT]" -ForegroundColor Green
    } else {
        Write-Host "[INFO] Nenhum role explicito no YAML existente. Assumindo: [CLIENT]" -ForegroundColor Gray
    }
} else {
    Write-Host "[INFO] Nenhuma configuracao previa encontrada. Usando modo padrao: [CLIENT]" -ForegroundColor Gray
}

# Sincroniza role no YAML se foi passado explicitamente
if ($EXPLICIT_MODE -ne "" -and (Test-Path $CONFIG_FILE)) {
    $content = Get-Content $CONFIG_FILE -Raw
    if ($content -match 'role:\s*.*') {
        $content = $content -replace 'role:\s*.*', "role: `"$MODE`""
    } else {
        $content = "role: `"$MODE`"`r`n" + $content
    }
    Set-Content -Path $CONFIG_FILE -Value $content -Encoding UTF8
    Write-Host "[CONFIG] Papel [$($MODE.ToUpper())] sincronizado e persistido em $CONFIG_FILE." -ForegroundColor Cyan
}

# 3. Detecta arquitetura do Windows
$osArch = (Get-CimInstance Win32_OperatingSystem).OSArchitecture
$ARCH = "amd64"
if ($osArch -match "ARM" -or $env:PROCESSOR_ARCHITECTURE -match "ARM") {
    $ARCH = "arm64"
}
Write-Host "[ARCH] Arquitetura detectada: $osArch -> windows-$ARCH" -ForegroundColor Gray

# 4. Parar tarefa existente se estiver rodando
function Stop-SupervisorTask {
    $task = Get-ScheduledTask -TaskName $TASK_NAME -ErrorAction SilentlyContinue
    if ($task) {
        Write-Host "[TASK] Interrompendo tarefa ativa do supervisor..." -ForegroundColor Gray
        Stop-ScheduledTask -TaskName $TASK_NAME -ErrorAction SilentlyContinue
        Start-Sleep -Seconds 1
    }
    Get-Process -Name "0itotrade-supervisor" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
}

# 5. Verificacao de Atualizacao Automatica
function Check-And-Update {
    if (Test-Path $BIN_PATH) {
        Write-Host "[UPDATE] Supervisor ja instalado em $BIN_PATH. Verificando atualizacoes no GitHub..." -ForegroundColor Cyan
        
        $currentVersion = "v0.0.0"
        try {
            $verOutput = & $BIN_PATH version 2>$null
            if ($verOutput -match "v([0-9]+\.[0-9]+\.[0-9]+)") {
                $currentVersion = "v" + $Matches[1]
            }
        } catch {}

        try {
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            $releaseInfo = Invoke-RestMethod -Uri $API_RELEASE_URL -Headers @{ "User-Agent" = "0itotrade-installer" } -TimeoutSec 10 -ErrorAction Stop
            $latestTag = $releaseInfo.tag_name

            if ($latestTag -and ($latestTag -ne $currentVersion)) {
                Write-Host ""
                Write-Host "[UPDATE] Nova versao disponivel no repositorio: $latestTag (Versao atual: $currentVersion)" -ForegroundColor Yellow
                
                $doUpdate = $true
                if ([Environment]::UserInteractive) {
                    $resp = Read-Host "Deseja atualizar para a versao $latestTag agora? [S/n]"
                    if ($resp -and ($resp.Trim().ToLower() -notmatch "^(s|sim|y|yes)$")) {
                        $doUpdate = $false
                    }
                }

                if ($doUpdate) {
                    Write-Host "[UPDATE] Atualizando binario do supervisor..." -ForegroundColor Cyan
                    Stop-SupervisorTask
                    
                    $tempZip = Join-Path $env:TEMP "0itotrade-supervisor-latest.zip"
                    $tempDir = Join-Path $env:TEMP "0itotrade-update-$(Get-Random)"
                    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
                    
                    $zipUrl = "https://github.com/$GITHUB_REPO/releases/latest/download/0itotrade-supervisor-windows-$ARCH.zip"
                    $exeUrl = "https://github.com/$GITHUB_REPO/releases/latest/download/0itotrade-supervisor-windows-$ARCH.exe"
                    $upSuccess = $false

                    try {
                        Invoke-WebRequest -Uri $zipUrl -OutFile $tempZip -UseBasicParsing -ErrorAction Stop
                        Expand-Archive -Path $tempZip -DestinationPath $tempDir -Force
                        $foundExe = Get-ChildItem -Path $tempDir -Filter "0itotrade-supervisor*.exe" -Recurse | Select-Object -First 1
                        if ($foundExe) {
                            Copy-Item -Path $foundExe.FullName -Destination $BIN_PATH -Force
                            $upSuccess = $true
                        }
                    } catch {
                        try {
                            Invoke-WebRequest -Uri $exeUrl -OutFile $BIN_PATH -UseBasicParsing -ErrorAction Stop
                            $upSuccess = $true
                        } catch {}
                    } finally {
                        Remove-Item -Path $tempZip -Force -ErrorAction SilentlyContinue
                        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
                    }

                    if ($upSuccess) {
                        Write-Host "[OK] Atualizacao para $latestTag concluida com sucesso!" -ForegroundColor Green
                        $task = Get-ScheduledTask -TaskName $TASK_NAME -ErrorAction SilentlyContinue
                        if ($task) {
                            Start-ScheduledTask -TaskName $TASK_NAME
                        }
                        exit 0
                    } else {
                        Write-Host "[AVISO] Falha ao baixar binario pre-compilado para Windows. Tentando compilacao local se Go estiver instalado..." -ForegroundColor Yellow
                    }
                } else {
                    Write-Host "[INFO] Atualizacao ignorada. Prosseguindo..." -ForegroundColor Gray
                }
            } else {
                Write-Host "[OK] O 0itotrade-supervisor ja esta na versao mais recente ($currentVersion)." -ForegroundColor Green
            }
        } catch {
            Write-Host "[INFO] Nao foi possivel consultar Releases do GitHub ($($_.Exception.Message)). Prosseguindo..." -ForegroundColor Gray
        }
    }
}

Check-And-Update

# 6. Criacao de Diretorios
Write-Host "[FS] Preparando diretorios do sistema..." -ForegroundColor Cyan
New-Item -ItemType Directory -Path $BIN_DIR -Force | Out-Null
New-Item -ItemType Directory -Path $CONFIG_DIR -Force | Out-Null
New-Item -ItemType Directory -Path $LOG_DIR -Force | Out-Null

# 7. Obtencao do Binario (Download ou Compilacao com Go)
$binaryInstalled = $false

if (Test-Path $BIN_PATH) {
    $binaryInstalled = $true
} else {
    Write-Host "[DOWNLOAD] Baixando binario 0itotrade-supervisor para windows-$ARCH do GitHub Releases..." -ForegroundColor Cyan
    
    $tempZip = Join-Path $env:TEMP "0itotrade-supervisor-download.zip"
    $tempDir = Join-Path $env:TEMP "0itotrade-dl-$(Get-Random)"
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

    $zipUrl = "https://github.com/$GITHUB_REPO/releases/latest/download/0itotrade-supervisor-windows-$ARCH.zip"
    $exeUrl = "https://github.com/$GITHUB_REPO/releases/latest/download/0itotrade-supervisor-windows-$ARCH.exe"

    try {
        Invoke-WebRequest -Uri $zipUrl -OutFile $tempZip -UseBasicParsing -ErrorAction Stop
        Expand-Archive -Path $tempZip -DestinationPath $tempDir -Force
        $foundExe = Get-ChildItem -Path $tempDir -Filter "0itotrade-supervisor*.exe" -Recurse | Select-Object -First 1
        if ($foundExe) {
            Copy-Item -Path $foundExe.FullName -Destination $BIN_PATH -Force
            $binaryInstalled = $true
        }
    } catch {
        try {
            Invoke-WebRequest -Uri $exeUrl -OutFile $BIN_PATH -UseBasicParsing -ErrorAction Stop
            $binaryInstalled = $true
        } catch {}
    } finally {
        Remove-Item -Path $tempZip -Force -ErrorAction SilentlyContinue
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    # Fallback: Compilacao local com Go se estiver instalado na maquina
    if (-not $binaryInstalled) {
        Write-Host "[AVISO] Binario pre-compilado nao encontrado no GitHub Releases." -ForegroundColor Yellow
        $goCmd = Get-Command go -ErrorAction SilentlyContinue
        $gitCmd = Get-Command git -ErrorAction SilentlyContinue

        if ($goCmd) {
            Write-Host "[BUILD] Compilando binario localmente via Go..." -ForegroundColor Cyan
            $srcDir = Join-Path $env:TEMP "0itotrade-src-$(Get-Random)"
            if ($gitCmd) {
                git clone --depth 1 "https://github.com/$GITHUB_REPO.git" $srcDir
            } else {
                $repoRoot = (Resolve-Path "$PSScriptRoot\..").Path
                if (Test-Path "$repoRoot\cmd\supervisor\main.go") {
                    $srcDir = $repoRoot
                }
            }

            if (Test-Path "$srcDir\cmd\supervisor\main.go") {
                Push-Location $srcDir
                try {
                    $env:CGO_ENABLED = "0"
                    go build -ldflags="-s -w" -o $BIN_PATH .\cmd\supervisor
                    if (Test-Path $BIN_PATH) {
                        $binaryInstalled = $true
                        Write-Host "[OK] Binario compilado com sucesso em $BIN_PATH!" -ForegroundColor Green
                    }
                } finally {
                    Pop-Location
                    if ($srcDir -like "$env:TEMP*") {
                        Remove-Item -Path $srcDir -Recurse -Force -ErrorAction SilentlyContinue
                    }
                }
            }
        }
    }
}

if (-not (Test-Path $BIN_PATH)) {
    Write-Host "[ERRO] Falha ao obter ou compilar o binario 0itotrade-supervisor.exe." -ForegroundColor Red
    exit 1
}

# 8. Adicionar ao PATH do Sistema (se ainda nao estiver)
$machinePath = [Environment]::GetEnvironmentVariable('Path', 'Machine')
if ($machinePath -notlike "*$BIN_DIR*") {
    Write-Host "[PATH] Adicionando $BIN_DIR ao PATH do Sistema..." -ForegroundColor Cyan
    $newPath = $machinePath + ';' + $BIN_DIR
    [Environment]::SetEnvironmentVariable('Path', $newPath, 'Machine')
    $env:Path = $env:Path + ';' + $BIN_DIR
}

# 9. Criacao da Configuracao Padrao se nao existir
if (-not (Test-Path $CONFIG_FILE)) {
    Write-Host "[CONFIG] Criando arquivo de configuracao padrao em $CONFIG_FILE para o modo [$($MODE.ToUpper())]..." -ForegroundColor Cyan
    $hostnameActual = [System.Net.Dns]::GetHostName()

    if ($MODE -eq "server") {
        $configLines = @(
            "# 0ITOTRADE-SUPERVISOR CONFIG (MODO SERVER / HUB)"
            "instance_name: `"$hostnameActual`""
            "role: `"server`""
            ""
            "server:"
            "  host: `"127.0.0.1`""
            "  port: 9100"
            "  auth_token: `"`""
            ""
            "mysql:"
            "  enabled: true"
            "  host: `"127.0.0.1`""
            "  port: 3306"
            "  user: `"0itotrade`""
            "  password: `"SEU_PASSWORD_AQUI`""
            "  database: `"0itotrade`""
            "  check_interval: 10s"
            "  timeout: 2000ms"
            "  max_latency_ms: 2000"
            ""
            "telegram:"
            "  enabled: false"
            "  bot_token: `"`""
            "  chat_id: `"`""
            "  daily_report_time: `"08:00`""
            "  alert_on_failure: true"
        )
    } else {
        $configLines = @(
            "# 0ITOTRADE-SUPERVISOR CONFIG (MODO CLIENT / AGENT)"
            "instance_name: `"$hostnameActual`""
            "role: `"client`""
            ""
            "server:"
            "  host: `"127.0.0.1`""
            "  port: 9100"
            "  auth_token: `"`""
            ""
            "hub:"
            "  host: `"132.145.56.248`""
            "  port: 8443"
            "  reconnect_interval: 5s"
            ""
            "telegram:"
            "  enabled: false"
            "  bot_token: `"`""
            "  chat_id: `"`""
            "  daily_report_time: `"08:00`""
            "  alert_on_failure: true"
        )
    }
    Set-Content -Path $CONFIG_FILE -Value ($configLines -join "`r`n") -Encoding UTF8
} else {
    Write-Host "[INFO] Arquivo de configuracao mantido em $CONFIG_FILE." -ForegroundColor Gray
}

# 10. Configuracao do Agendador de Tarefas do Windows (Task Scheduler)
Write-Host "[TASK] Configurando tarefa agendada de execucao automatica ($TASK_NAME)..." -ForegroundColor Cyan
Stop-SupervisorTask

$action = New-ScheduledTaskAction -Execute $BIN_PATH -Argument "run -config `"$CONFIG_FILE`""
$trigger = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -MultipleInstances Parallel -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit (New-TimeSpan -Days 0)
$principal = New-ScheduledTaskPrincipal -UserId "NT AUTHORITY\SYSTEM" -LogonType ServiceAccount -RunLevel Highest

Register-ScheduledTask -TaskName $TASK_NAME -Action $action -Trigger $trigger -Settings $settings -Principal $principal -Force | Out-Null

Write-Host "[TASK] Iniciando tarefa agendada 0itotrade-supervisor..." -ForegroundColor Cyan
Start-ScheduledTask -TaskName $TASK_NAME

Start-Sleep -Seconds 2

# 11. Verificacao de Status
Write-Host "========================================================" -ForegroundColor Green
Write-Host " [OK] Instalacao / Atualizacao no Windows Concluida com Sucesso!" -ForegroundColor Green
Write-Host "========================================================" -ForegroundColor Green

$taskInfo = Get-ScheduledTask -TaskName $TASK_NAME -ErrorAction SilentlyContinue
Write-Host "• Status da Tarefa no Windows: $($taskInfo.State)" -ForegroundColor Cyan
Write-Host "• Executavel: $BIN_PATH" -ForegroundColor Gray
Write-Host "• Configuracao: $CONFIG_FILE" -ForegroundColor Gray

try {
    $healthResp = Invoke-RestMethod -Uri "http://127.0.0.1:9100/health" -TimeoutSec 3 -ErrorAction SilentlyContinue
    if ($healthResp) {
        Write-Host "• Telemetria Local HTTP: OK (Instancia: $($healthResp.instance_name), Status: $($healthResp.status))" -ForegroundColor Green
    }
} catch {
    Write-Host "• Telemetria Local HTTP: (Servico iniciando em segundo plano...)" -ForegroundColor Yellow
}

Write-Host ""
Write-Host "Proximos Passos no Windows:" -ForegroundColor Cyan
Write-Host "1. Edite as credenciais e configuracoes em:"
Write-Host "   notepad `"$CONFIG_FILE`""
Write-Host "2. Reinicie a tarefa agendada:"
Write-Host "   Stop-ScheduledTask -TaskName $TASK_NAME; Start-ScheduledTask -TaskName $TASK_NAME"
Write-Host "3. Verifique o status pelo terminal:"
Write-Host "   0itotrade-supervisor status"
Write-Host "========================================================" -ForegroundColor Green
