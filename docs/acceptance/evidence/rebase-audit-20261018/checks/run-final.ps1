$ErrorActionPreference='Stop'
while((Get-Content .evidence/rebase-audit-20261018/full/summary.json -Raw|ConvertFrom-Json).status -eq 'running'){Start-Sleep -Seconds 2}
pwsh -NoProfile -File scripts/test.ps1 -Level Controlled -OutDir .evidence/rebase-audit-20261018/full-final
exit $LASTEXITCODE
