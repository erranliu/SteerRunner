param([string]$FixtureRoot,[ValidateSet('empty','existing','conflict','query-failure')][string]$Case)
$ErrorActionPreference = 'Stop'
Set-Location -LiteralPath $FixtureRoot
$script:MockCase = $Case
$script:MockLog = Join-Path $FixtureRoot 'calls.log'
function git {
    [IO.File]::AppendAllText($script:MockLog,('git ' + ($args -join ' ') + "`n"))
    $global:LASTEXITCODE = 0
    switch ($args -join ' ') {
        'remote get-url origin' { 'https://github.com/erranliu/SteerlineTest.git' }
        'branch --show-current' { 'codex/qa-head' }
        'status --porcelain' { }
        'push origin HEAD:refs/heads/codex/qa-head' { }
        default { throw 'Unexpected Git invocation in publisher test' }
    }
}
function gh {
    [IO.File]::AppendAllText($script:MockLog,('gh ' + ($args -join ' ') + "`n"))
    $global:LASTEXITCODE = 0
    if ($args[0] -eq 'pr' -and $args[1] -eq 'list') {
        switch ($script:MockCase) {
            'empty' { '[]' }
            'existing' { '[{"url":"https://github.com/erranliu/SteerlineTest/pull/1","state":"OPEN"}]' }
            'conflict' { '[{"state":"OPEN"},{"state":"CLOSED"}]' }
            'query-failure' { $global:LASTEXITCODE = 1 }
        }
    } elseif ($args[0] -eq 'pr' -and $args[1] -eq 'create' -and $script:MockCase -eq 'empty') {
        'https://github.com/erranliu/SteerlineTest/pull/2'
    } else { throw 'Publisher attempted an unexpected or duplicate PR creation' }
}
. (Join-Path $FixtureRoot 'sr-smoke/publish.ps1')
