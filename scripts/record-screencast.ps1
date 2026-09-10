param(
    # Where the web app is reachable. Start the OIDC overlay first:
    #   docker compose -f compose.yml -f compose.oidc.yml up --build
    [string]$BaseUrl = "http://localhost:3000",
    # Output width in pixels; height follows the 16:9 recording.
    [int]$Width = 960,
    # Frames per second of the GIF. Lower is smaller; 8 keeps page transitions readable.
    [int]$Fps = 8,
    # Skip the scoped demo reset before recording (the reset makes the learner start un-enrolled).
    [switch]$NoReset
)
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$web = Join-Path $root "web"
$out = Join-Path $root "docs\assets\recruiter-walkthrough.gif"

if (-not (Get-Command ffmpeg -ErrorAction SilentlyContinue)) {
    throw "ffmpeg is required on PATH to render the GIF"
}

if (-not $NoReset) {
    # Start from a clean learner: the recording enrolls and completes lessons itself.
    docker compose -f (Join-Path $root "compose.yml") -f (Join-Path $root "compose.oidc.yml") `
        exec -T -e RESET_CONFIRM=synthetic-demo api /resetdemo
    if ($LASTEXITCODE -ne 0) { throw "demo reset failed" }
}

Push-Location $web
try {
    $results = Join-Path $web "test-results"
    if (Test-Path $results) { Remove-Item -Recurse -Force $results }
    $env:PLAYWRIGHT_BASE_URL = $BaseUrl
    $env:PLAYWRIGHT_SCREENCAST = "1"
    npx playwright test tests/screencast.spec.ts --workers=1 --retries=0
    if ($LASTEXITCODE -ne 0) { throw "screencast recording failed" }

    $video = Get-ChildItem -Path $results -Recurse -Filter "*.webm" | Select-Object -First 1
    if (-not $video) { throw "no video was recorded under $results" }

    # Two-pass palette render: a per-clip palette with Bayer dithering keeps flat UI colours
    # crisp at a fraction of the size of a generic 256-colour conversion.
    $filters = "fps=$Fps,scale=${Width}:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=128:stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=5:diff_mode=rectangle"
    ffmpeg -y -loglevel error -i $video.FullName -vf $filters -loop 0 $out
    if ($LASTEXITCODE -ne 0) { throw "ffmpeg render failed" }

    $mb = [math]::Round((Get-Item $out).Length / 1MB, 2)
    Write-Host "wrote $out ($mb MB) from $($video.FullName)"
    Remove-Item -Recurse -Force $results
}
finally {
    Remove-Item Env:\PLAYWRIGHT_SCREENCAST -ErrorAction SilentlyContinue
    Pop-Location
}
