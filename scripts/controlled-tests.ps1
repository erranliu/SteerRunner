# Test selection and diagnostic timings only; never creates prerequisite proof.
function Assert-SRControlledSelectionCatalog($Catalog) {
    $registered = @($Catalog.controlled_scenarios)
    $all = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($test in $registered) {
        if ($test -cnotmatch '^Test[A-Za-z0-9_]+$' -or -not $all.Add($test)) { throw 'Invalid or duplicate Controlled test' }
    }
    if (-not $all.Count) { throw 'Controlled tests must not be empty' }
    $owned = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($kind in @('groups','combinations')) {
        $definitions = @($Catalog.controlled_selection.$kind.PSObject.Properties)
        if (-not $definitions.Count) { throw "Controlled $kind must not be empty" }
        foreach ($definition in $definitions) {
            if ($definition.Name -cnotmatch '^[a-z][a-z0-9-]+$' -or -not @($definition.Value).Count) { throw "Invalid Controlled $kind definition" }
            $seen = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
            foreach ($test in @($definition.Value)) {
                if (-not $all.Contains($test) -or -not $seen.Add($test)) { throw "Unknown or duplicate test in Controlled $kind/$($definition.Name): $test" }
                if ($kind -eq 'groups' -and -not $owned.Add($test)) { throw "Controlled test has multiple primary groups: $test" }
            }
        }
    }
    if (-not $owned.SetEquals($all)) { throw 'Controlled groups must cover every registered test exactly once' }
}

function Resolve-SRControlledSelection($Catalog, [string[]]$Groups=@(), [string[]]$Combinations=@()) {
    Assert-SRControlledSelectionCatalog $Catalog
    if (($Groups.Count -eq 0) -eq ($Combinations.Count -eq 0)) { throw 'Select either Group or Combination, with at least one exact ID' }
    $kind = if ($Groups.Count) { 'groups' } else { 'combinations' }
    $ids = if ($Groups.Count) { $Groups } else { $Combinations }
    $selected = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    $seen = [Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($id in $ids) {
        $definition = @($Catalog.controlled_selection.$kind.PSObject.Properties | Where-Object {$_.Name -ceq $id})
        if ($definition.Count -ne 1 -or -not $seen.Add($id)) { throw "Unknown or duplicate Controlled selection: $id" }
        foreach ($test in @($definition[0].Value)) { $null = $selected.Add($test) }
    }
    $tests = @($Catalog.controlled_scenarios | Where-Object {$selected.Contains($_)})
    if (-not $tests.Count) { throw 'No Controlled tests selected' }
    @{kind=$kind;ids=@($ids);tests=$tests;pattern=('^('+(($tests | ForEach-Object {[regex]::Escape($_)}) -join '|')+')$')}
}

function Get-SRControlledTimingReport($Catalog, [string]$Directory, [string[]]$ExpectedTests=@()) {
    Assert-SRControlledSelectionCatalog $Catalog
    $directoryPath = (Resolve-Path -LiteralPath $Directory).Path
    $logPath = Join-Path $directoryPath 'integration.log'
    $records = [Collections.Generic.Dictionary[string,object]]::new([StringComparer]::Ordinal)
    $errors = [Collections.Generic.List[string]]::new()
    # A top-level record owns its subtests' time. Subtest lines are never summed.
    foreach ($line in [IO.File]::ReadAllLines($logPath)) {
        if ($line -cmatch '^=== RUN   (Test[A-Za-z0-9_]+)$') {
            $name = $Matches[1]
            if ($records.ContainsKey($name)) { $errors.Add("Duplicate test run: $name"); continue }
            $records[$name] = @{test=$name;status='incomplete';duration_seconds=$null}
        } elseif ($line -cmatch '^--- (PASS|FAIL|SKIP): (Test[A-Za-z0-9_]+) \(([0-9]+(?:\.[0-9]+)?)s\)$') {
            $name = $Matches[2]; $status = $Matches[1].ToLowerInvariant()
            $seconds = [double]::Parse($Matches[3], [Globalization.CultureInfo]::InvariantCulture)
            if (-not $records.ContainsKey($name)) {
                $errors.Add("Result without start: $name")
                $records[$name] = @{test=$name;status='incomplete';duration_seconds=$null}
            }
            if ($records[$name].status -ne 'incomplete') { $errors.Add("Duplicate test result: $name"); continue }
            $records[$name].status=$status; $records[$name].duration_seconds=$seconds
        }
    }
    $tests = @(foreach ($record in $records.Values) {
        $group = @($Catalog.controlled_selection.groups.PSObject.Properties | Where-Object {@($_.Value) -ccontains $record.test})
        $record['group'] = if ($group.Count -eq 1) { $group[0].Name } else { $null }
        $record
    })
    if (-not $tests.Count) { $errors.Add('No top-level tests observed') }
    $incomplete = @($tests | Where-Object {$_.status -eq 'incomplete'} | ForEach-Object {$_.test})
    $unknown = @($tests | Where-Object {$null -eq $_.group} | ForEach-Object {$_.test})
    $missing = @($ExpectedTests | Where-Object { -not $records.ContainsKey($_) })
    $unexpected = @(if ($ExpectedTests.Count) { $tests | Where-Object {$ExpectedTests -cnotcontains $_.test} | ForEach-Object {$_.test} })
    $resultPath = Join-Path $directoryPath 'result.json'
    $result = $null
    if (Test-Path -LiteralPath $resultPath) {
        $result = Get-Content -LiteralPath $resultPath -Raw | ConvertFrom-Json
        if ($result.exit_code -ne 0 -or $result.scenario_count -ne $tests.Count) { $errors.Add('Result exit/count differs from observed top-level tests') }
    } else { $errors.Add('Missing harness result.json') }
    $groups = @(foreach ($definition in $Catalog.controlled_selection.groups.PSObject.Properties) {
        $observed = @($tests | Where-Object {$_.group -ceq $definition.Name})
        $measured = @($observed | Where-Object {$null -ne $_.duration_seconds})
        $sum = if ($measured.Count) { ($measured | Measure-Object -Property duration_seconds -Sum).Sum } else { 0 }
        @{id=$definition.Name;registered_count=@($definition.Value).Count;observed_count=$observed.Count;measured_count=$measured.Count;
          missing_tests=@($definition.Value | Where-Object {-not $records.ContainsKey($_)});
          observed_seconds=[Math]::Round([double]$sum,2)}
    })
    $measured = @($tests | Where-Object {$null -ne $_.duration_seconds})
    $totalSeconds = if ($measured.Count) { ($measured | Measure-Object -Property duration_seconds -Sum).Sum } else { 0 }
    $passed = @($tests | Where-Object {$_.status -eq 'pass'}).Count
    $status = if ($errors.Count -or $missing.Count -or $unexpected.Count -or $unknown.Count -or $incomplete.Count -or $passed -ne $tests.Count) { 'incomplete_or_failed' } else { 'passed' }
    $sources = @(foreach ($name in @('integration.log','result.json','source-identity.json')) {
        $path = Join-Path $directoryPath $name
        if (Test-Path -LiteralPath $path) { @{path=$path;sha256=(Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant()} }
    })
    @{schema='steerrunner-controlled-timings/v1';validation_scope='controlled-timing';complete_controlled_passed=$false;qualification_eligible=$false;
      status=$status;evidence=$directoryPath;sources=$sources;expected_tests=$ExpectedTests;observed_count=$tests.Count;passed_count=$passed;
      missing_tests=$missing;unexpected_tests=$unexpected;unknown_tests=$unknown;incomplete_tests=$incomplete;errors=@($errors.ToArray());
      observed_top_level_seconds=[Math]::Round([double]$totalSeconds,2);
      tests=@($tests | Sort-Object test);groups=$groups;slowest=@($measured | Sort-Object @{Expression='duration_seconds';Descending=$true},test | Select-Object -First 10)}
}
