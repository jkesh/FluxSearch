# FluxSearch Windows 开发脚本（PowerShell）
# 用法: .\scripts\dev.ps1 api | frontend | build

param(
    [Parameter(Position = 0)]
    [ValidateSet("api", "frontend", "build", "build-frontend", "test")]
    [string]$Target = "api"
)

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

switch ($Target) {
    "api" {
        Write-Host "Starting fluxsearch-api on :8080 ..."
        go run ./cmd/api
    }
    "frontend" {
        Write-Host "Starting frontend dev server on :5173 ..."
        Set-Location frontend
        npm run dev
    }
    "build" {
        New-Item -ItemType Directory -Force -Path bin | Out-Null
        go build -o bin/fluxsearch-api.exe ./cmd/api
        go build -o bin/fluxsearch-worker.exe ./cmd/worker
        Write-Host "Built: bin/fluxsearch-api.exe, bin/fluxsearch-worker.exe"
    }
    "build-frontend" {
        Set-Location frontend
        npm run build
    }
    "test" {
        go test ./...
    }
}
