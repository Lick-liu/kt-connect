# 将本仓库最新 router 修复热应用到运行中的 kt router pod（无需构建/推送镜像）。
# 用法: pwsh -File hack/patch-router.ps1 [-Namespace dev] [-Service tea-shop-merchant-be-biz]
# 原理: route.conf 模板经 go:embed 编译进 router 二进制; kubectl cp 替换二进制后
#       执行一次幂等 add 即可触发 prune + 新模板重写 + nginx -t 校验 + graceful reload。
param(
    [string]$Namespace = "dev",
    [string]$Service = "tea-shop-merchant-be-biz"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
$routerPod = "$Service-kt-router"
$binary = Join-Path $repoRoot "artifacts/router/router-linux-amd64"

Write-Host "[1/4] 编译 linux router 二进制 ..."
Push-Location $repoRoot
try {
    $env:CGO_ENABLED = "0"; $env:GOOS = "linux"; $env:GOARCH = "amd64"
    go build -o artifacts/router/router-linux-amd64 cmd/router/main.go
    if ($LASTEXITCODE -ne 0) { throw "go build failed" }
} finally {
    Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
    Pop-Location
}

Write-Host "[2/4] 替换 $Namespace/$routerPod 内的 /usr/sbin/router ..."
kubectl cp $binary "${Namespace}/${routerPod}:/tmp/router.new"
if ($LASTEXITCODE -ne 0) { throw "kubectl cp failed" }
kubectl -n $Namespace exec $routerPod -- sh -c "mv /tmp/router.new /usr/sbin/router && chmod +x /usr/sbin/router"
if ($LASTEXITCODE -ne 0) { throw "binary install failed" }

Write-Host "[3/4] 读取 kt.conf 并以现有版本触发幂等 add（重写为新模板并 reload）..."
$ktConfRaw = kubectl -n $Namespace exec $routerPod -- cat /etc/kt.conf
if ($LASTEXITCODE -ne 0) { throw "read /etc/kt.conf failed (router 尚未 setup?)" }
$ktConf = $ktConfRaw | ConvertFrom-Json
if (-not $ktConf.Versions -or $ktConf.Versions.Count -eq 0) { throw "kt.conf has no versions" }
$mark = "$($ktConf.Header):$($ktConf.Versions[0])"
kubectl -n $Namespace exec $routerPod -- /usr/sbin/router add $mark
if ($LASTEXITCODE -ne 0) { throw "router add failed" }

Write-Host "[4/4] 校验 ..."
kubectl -n $Namespace exec $routerPod -- sh -c "nginx -t 2>&1 | tail -1; grep -c kt_fallback /etc/nginx/conf.d/route.conf"
if ($LASTEXITCODE -ne 0) { throw "verification failed" }
Write-Host "完成: $Namespace/$routerPod 已运行带 fallback 的新路由配置。"
