function Convert-SRPRList([string]$JSON) {
    if (-not $JSON.TrimStart().StartsWith('[')) { throw 'PR query did not return an array' }
    $taskDecoded = ConvertFrom-Json -InputObject $JSON
    # Windows PowerShell 5 emits the empty JSON array as one pipeline object.
    # Enumerate explicitly so an empty list cannot look like a conflicting PR.
    foreach ($taskPR in $taskDecoded) {
        if ($null -ne $taskPR) { Write-Output $taskPR }
    }
}
