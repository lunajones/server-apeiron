param(
    [switch]$Build,
    [switch]$Restart,
    [switch]$MovementValidation,
    [string]$DbEndpoint = "127.0.0.1:50051"
)

$ErrorActionPreference = "Stop"

$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
$Bin = Join-Path $Root "bin\game-server.exe"
$LogDir = Join-Path $Root "logs"
$Stamp = Get-Date -Format "yyyyMMdd_HHmmss"

New-Item -ItemType Directory -Force -Path (Split-Path $Bin) | Out-Null
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

if ($Restart) {
    Get-Process -Name "game-server" -ErrorAction SilentlyContinue | Stop-Process -Force
}

if ($Build -or -not (Test-Path $Bin)) {
    Push-Location $Root
    try {
        go build -o $Bin ./cmd/game-server
    } finally {
        Pop-Location
    }
}

$env:DB_APEIRON_ENDPOINT = $DbEndpoint
$env:DB_APEIRON_STARTUP_REQUIRED = "true"
$env:DB_APEIRON_STARTUP_WARMUP_ENABLED = "true"
$env:STATICDATA_PRELOAD_CREATURE_TEMPLATE_IDS = "steppe_wolf"
$env:STATICDATA_PRELOAD_SKILL_IDS = "player_basic_attack_1,player_basic_attack_2,player_basic_attack_3,player_shield_bash,player_shield_rush,bite,lunge,wolf_dodge,maul"
$env:STATICDATA_PRELOAD_SKILL_SET_IDS = "skillset_steppe_wolf"
$env:STATICDATA_PRELOAD_WEAPON_KIT_IDS = "weaponkit_sword_shield"
$env:MOVEMENT_VALIDATION = if ($MovementValidation) { "true" } else { "false" }

$OutLog = Join-Path $LogDir "game-server-db-$Stamp.out.log"
$ErrLog = Join-Path $LogDir "game-server-db-$Stamp.err.log"

Start-Process -FilePath $Bin -WorkingDirectory $Root -RedirectStandardOutput $OutLog -RedirectStandardError $ErrLog -WindowStyle Hidden
Write-Host "game-server started with DB_APEIRON_ENDPOINT=$DbEndpoint MOVEMENT_VALIDATION=$env:MOVEMENT_VALIDATION"
Write-Host "stdout: $OutLog"
Write-Host "stderr: $ErrLog"
