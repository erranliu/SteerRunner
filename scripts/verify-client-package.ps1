param([Parameter(Mandatory=$true)][string]$PackageDir,[string]$Qualification='')
$ErrorActionPreference = 'Stop'
Set-StrictMode -Version 2
. (Join-Path $PSScriptRoot 'delivery-contract.ps1')
$taskIdentity = if ($Qualification) { Test-SRDeliveryQualification $PackageDir $Qualification } else { Test-SRClientPackage $PackageDir }
@{schema='steerrunner-package-verification/v1';package=$taskIdentity;qualification_verified=[bool]$Qualification} | ConvertTo-Json -Depth 8
