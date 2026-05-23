Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

$ScriptDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }
$ExePath = Join-Path $ScriptDir "claude-key-proxy.exe"
$ConfigPath = Join-Path $ScriptDir "config.json"
$LogDir = Join-Path $ScriptDir "logs"
$StdoutLog = Join-Path $LogDir "startup.stdout.log"
$StderrLog = Join-Path $LogDir "startup.stderr.log"
$PidFile = Join-Path $LogDir "claude-key-proxy.pid"

# 输出统一前缀，方便从双击窗口或命令行快速判断启动结果。
function Write-StartupInfo {
    param([string]$Message)
    Write-Host "[claude-key-proxy] $Message"
}

# 从 config.json 解析 admin 服务地址；未配置时使用程序默认值。
function Get-AdminBaseUrl {
    param([string]$Path)

    $adminListen = "127.0.0.1:8081"
    if (Test-Path -LiteralPath $Path) {
        $cfg = Get-Content -LiteralPath $Path -Raw | ConvertFrom-Json
        if ($cfg.admin_listen) {
            $adminListen = [string]$cfg.admin_listen
        }
    }

    $hostName = "127.0.0.1"
    $port = "8081"
    if ($adminListen -match "^:(\d+)$") {
        $port = $Matches[1]
    } elseif ($adminListen -match "^(.+):(\d+)$") {
        $hostName = $Matches[1]
        $port = $Matches[2]
    }

    if ($hostName -eq "0.0.0.0" -or $hostName -eq "::" -or $hostName -eq "[::]") {
        $hostName = "127.0.0.1"
    }
    return "http://$hostName`:$port"
}

# 从 Invoke-WebRequest 的异常中安全提取 HTTP 状态码；连接失败等异常没有 Response 属性。
function Get-ErrorHttpStatusCode {
    param([System.Management.Automation.ErrorRecord]$ErrorRecord)

    $exception = $ErrorRecord.Exception
    if ($null -eq $exception) {
        return $null
    }

    $responseProperty = $exception.PSObject.Properties["Response"]
    if ($null -ne $responseProperty -and $null -ne $responseProperty.Value) {
        $statusProperty = $responseProperty.Value.PSObject.Properties["StatusCode"]
        if ($null -ne $statusProperty -and $null -ne $statusProperty.Value) {
            return [int]$statusProperty.Value
        }
    }

    $statusCodeProperty = $exception.PSObject.Properties["StatusCode"]
    if ($null -ne $statusCodeProperty -and $null -ne $statusCodeProperty.Value) {
        return [int]$statusCodeProperty.Value
    }

    return $null
}

# 只要 admin health 端点有 HTTP 响应，就认为服务已经在运行，避免重复启动。
function Test-AdminResponding {
    param([string]$BaseUrl)

    try {
        $resp = Invoke-WebRequest -Uri "$BaseUrl/admin/health" -UseBasicParsing -TimeoutSec 2
        return [pscustomobject]@{ Responding = $true; StatusCode = [int]$resp.StatusCode }
    } catch {
        $statusCode = Get-ErrorHttpStatusCode -ErrorRecord $_
        if ($null -ne $statusCode) {
            return [pscustomobject]@{ Responding = $true; StatusCode = $statusCode }
        }
        return [pscustomobject]@{ Responding = $false; StatusCode = 0 }
    }
}

# 本地没有二进制时自动构建，保持脚本一键可用。
function Ensure-Binary {
    if (Test-Path -LiteralPath $ExePath) {
        return
    }
    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Missing $ExePath and go command is not available for auto build."
    }

    Write-StartupInfo "claude-key-proxy.exe not found, building..."
    Push-Location $ScriptDir
    try {
        & go build -trimpath -ldflags="-s -w" -o $ExePath .
    } finally {
        Pop-Location
    }
}

# 后台启动独立进程；父终端关闭后该进程仍会继续运行。
function Start-ProxyProcess {
    New-Item -ItemType Directory -Path $LogDir -Force | Out-Null
    $arguments = @("-config", "`"$ConfigPath`"")
    return Start-Process `
        -FilePath $ExePath `
        -ArgumentList $arguments `
        -WorkingDirectory $ScriptDir `
        -WindowStyle Hidden `
        -RedirectStandardOutput $StdoutLog `
        -RedirectStandardError $StderrLog `
        -PassThru
}

if (-not (Test-Path -LiteralPath $ConfigPath)) {
    throw "Missing config file: $ConfigPath"
}

$adminBaseUrl = Get-AdminBaseUrl -Path $ConfigPath
$existing = Test-AdminResponding -BaseUrl $adminBaseUrl
if ($existing.Responding) {
    Write-StartupInfo "service is already running: $adminBaseUrl, health status $($existing.StatusCode)."
    exit 0
}

Ensure-Binary
$process = Start-ProxyProcess
Set-Content -LiteralPath $PidFile -Value $process.Id -Encoding ASCII
Write-StartupInfo "started in background, PID $($process.Id)."

for ($i = 0; $i -lt 20; $i++) {
    Start-Sleep -Milliseconds 500
    if ($process.HasExited) {
        throw "service process exited with code $($process.ExitCode). Check $StderrLog"
    }

    $health = Test-AdminResponding -BaseUrl $adminBaseUrl
    if ($health.Responding) {
        Write-StartupInfo "ready: $adminBaseUrl, health status $($health.StatusCode). You can close this terminal."
        exit 0
    }
}

Write-StartupInfo "background process started but health is not responding yet. PID $($process.Id). Check $adminBaseUrl/admin/health later."
