# Test selection and diagnostic timings only; never creates prerequisite proof.
function Get-SRControlledCaseDefinitions {
    [ordered]@{
        TestSRRestartPreservesAuthorization=@('graceful','kill','creation_receipt','pending_stop')
        TestSRResultFile=@('optional-file-failure-business-loop','final-seal')
        TestSRResultAcquisition=@('plain-marker','audit-zero','audit-positive','audit-offline','audit-report','parser-unsupported','parser-complete','parser-tool')
    }
}

function Assert-SRControlledCaseSelection([string]$Parent,[string[]]$Cases) {
    $definitions=Get-SRControlledCaseDefinitions
    if ($Parent -cnotin @($definitions.Keys) -or -not $Cases.Count) { throw 'Unknown controlled parent or empty case selection' }
    $seen=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($id in $Cases) {
        if (@($definitions[$Parent]) -cnotcontains $id -or -not $seen.Add($id)) { throw "Unknown or duplicate controlled case: $Parent/$id" }
    }
}

function ConvertTo-SRControlledCasePlan([string[]]$Paths) {
    $plan=@{}
    foreach ($path in $Paths) {
        $parts=@($path.Split('/'))
        if ($parts.Count -ne 2) { throw "Controlled case requires exact parent/case: $path" }
        $parent=$parts[0]; $id=$parts[1]
        Assert-SRControlledCaseSelection $parent @($id)
        if (-not $plan.ContainsKey($parent)) { $plan[$parent]=@() }
        if ($plan[$parent] -ccontains $id) { throw "Duplicate controlled case: $path" }
        $plan[$parent]+=@($id)
    }
    $plan
}

function Get-SRResultFileCaseIDs { @((Get-SRControlledCaseDefinitions)['TestSRResultFile']) }
function Assert-SRResultFileCaseSelection([string[]]$Cases) { Assert-SRControlledCaseSelection 'TestSRResultFile' $Cases }

function Read-SRControlledSubtests([string]$Directory) {
    $records=[Collections.Generic.Dictionary[string,object]]::new([StringComparer]::Ordinal)
    $errors=[Collections.Generic.List[string]]::new()
    foreach ($line in [IO.File]::ReadAllLines((Join-Path $Directory 'integration.log'))) {
        if ($line -cmatch '^=== RUN   (Test[^ /]+/[^ ]+)$') {
            $name=$Matches[1]
            if ($records.ContainsKey($name)) { $errors.Add("Duplicate subtest run: $name"); continue }
            $records[$name]=@{test=$name;status='incomplete';duration_seconds=$null}
        } elseif ($line -cmatch '^\s*--- (PASS|FAIL|SKIP): (Test[^ /]+/[^ ]+) \(([0-9]+(?:\.[0-9]+)?)s\)$') {
            $name=$Matches[2]; $status=$Matches[1].ToLowerInvariant()
            $seconds=[double]::Parse($Matches[3],[Globalization.CultureInfo]::InvariantCulture)
            if (-not $records.ContainsKey($name)) { $errors.Add("Subtest result without start: $name"); continue }
            if ($records[$name].status -ne 'incomplete') { $errors.Add("Duplicate subtest result: $name"); continue }
            $records[$name].status=$status; $records[$name].duration_seconds=$seconds
        }
    }
    @{records=@($records.Values | Sort-Object test);errors=@($errors.ToArray())}
}

function Get-SRControlledCaseEvidence([string]$Directory,[string]$Parent,[string[]]$ExpectedCases) {
    Assert-SRControlledCaseSelection $Parent $ExpectedCases
    $label=$Parent.Substring(6)
    $result=Get-Content -LiteralPath (Join-Path $Directory 'result.json') -Raw | ConvertFrom-Json
    if ($Parent -ceq 'TestSRResultFile') {
        if (-not $result.PSObject.Properties['result_file_cases']) { throw 'Missing actual ResultFile case receipts' }
        $receipts=@($result.result_file_cases)
        if ($result.PSObject.Properties['controlled_cases']) {
            if (-not $result.controlled_cases.PSObject.Properties[$Parent] -or
                ($receipts | ConvertTo-Json -Depth 8 -Compress) -cne (@($result.controlled_cases.$Parent) | ConvertTo-Json -Depth 8 -Compress)) { throw 'ResultFile legacy and per-parent receipts differ' }
        }
    } else {
        if (-not $result.PSObject.Properties['controlled_cases'] -or -not $result.controlled_cases.PSObject.Properties[$Parent]) { throw "Missing actual $label case receipts" }
        $receipts=@($result.controlled_cases.$Parent)
    }
    $actual=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
    foreach ($receipt in $receipts) {
        if (-not $actual.Add($receipt.id) -or $receipt.passed -isnot [bool] -or -not $receipt.passed -or
            ($receipt.duration_seconds -isnot [int] -and $receipt.duration_seconds -isnot [long] -and $receipt.duration_seconds -isnot [double] -and $receipt.duration_seconds -isnot [decimal]) -or [double]$receipt.duration_seconds -lt 0 -or
            [double]::IsNaN([double]$receipt.duration_seconds) -or [double]::IsInfinity([double]$receipt.duration_seconds)) { throw "Invalid actual $label case receipt" }
    }
    if (-not $actual.SetEquals($ExpectedCases)) { throw "Actual $label cases differ from the exact selection" }
    $subtests=Read-SRControlledSubtests $Directory
    if ($subtests.errors.Count) { throw ($subtests.errors -join '; ') }
    $required=@(foreach ($id in $ExpectedCases) {
        "$Parent/$id"
        if ($Parent -ceq 'TestSRResultFile') { foreach ($mask in 0..3) { "$Parent/$id/enabled-$mask" } }
    })
    $observed=@($subtests.records | Where-Object {$_.test.StartsWith($Parent+'/',[StringComparison]::Ordinal)})
    if ($observed.Count -ne $required.Count -or @($observed | Where-Object {$required -cnotcontains $_.test -or $_.status -ne 'pass'}).Count -or
        @($required | Where-Object {$_ -cnotin @($observed.test)}).Count) { throw "Missing, failed, skipped or unexpected $label subcases/enable variants" }
    @{status='passed';expected_cases=$ExpectedCases;actual_cases=@($receipts);required_paths=$required}
}

function Get-SRResultFileCaseEvidence([string]$Directory,[string[]]$ExpectedCases) {
    Get-SRControlledCaseEvidence $Directory 'TestSRResultFile' $ExpectedCases
}

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
    if ($Catalog.controlled_selection.PSObject.Properties['result_file_cases']) {
        foreach ($definition in $Catalog.controlled_selection.result_file_cases.PSObject.Properties) {
            if ($definition.Name -cnotin @($Catalog.controlled_selection.combinations.PSObject.Properties.Name)) { throw 'ResultFile cases reference an unknown combination' }
            if (@($Catalog.controlled_selection.combinations.($definition.Name)) -cnotcontains 'TestSRResultFile') { throw 'ResultFile cases require a combination containing TestSRResultFile' }
            Assert-SRResultFileCaseSelection @($definition.Value)
        }
    }
    if ($Catalog.controlled_selection.PSObject.Properties['subcases']) {
        foreach ($definition in $Catalog.controlled_selection.subcases.PSObject.Properties) {
            if ($definition.Name -cnotin @($Catalog.controlled_selection.combinations.PSObject.Properties.Name)) { throw 'Subcases reference an unknown combination' }
            foreach ($parent in $definition.Value.PSObject.Properties) {
                if (@($Catalog.controlled_selection.combinations.($definition.Name)) -cnotcontains $parent.Name) { throw 'Subcases require their parent in the combination' }
                Assert-SRControlledCaseSelection $parent.Name @($parent.Value)
            }
        }
    }
}

function Resolve-SRControlledSelection($Catalog, [string[]]$Groups=@(), [string[]]$Combinations=@(), [string[]]$ResultFileCases=@(), [string[]]$Cases=@()) {
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
    $explicit=ConvertTo-SRControlledCasePlan $Cases
    if ($ResultFileCases.Count) {
        Assert-SRResultFileCaseSelection $ResultFileCases
        if ($explicit.ContainsKey('TestSRResultFile')) { throw 'ResultFile cannot use both Case and ResultFileCase' }
        $explicit['TestSRResultFile']=$ResultFileCases
    }
    foreach ($parent in $explicit.Keys) {
        if ($tests -cnotcontains $parent) { throw "Case requires its parent in selected tests: $parent" }
    }
    $plan=@{}
    $definitions=Get-SRControlledCaseDefinitions
    foreach ($parent in $definitions.Keys) {
        if ($tests -cnotcontains $parent) { continue }
        if ($explicit.ContainsKey($parent)) { $plan[$parent]=@($definitions[$parent] | Where-Object {$explicit[$parent] -ccontains $_}); continue }
        $wanted=[Collections.Generic.HashSet[string]]::new([StringComparer]::Ordinal)
        foreach ($id in $ids) {
            if (@($Catalog.controlled_selection.$kind.$id) -cnotcontains $parent) { continue }
            $narrow=@()
            if ($kind -eq 'combinations') {
                if ($Catalog.controlled_selection.PSObject.Properties['subcases']) {
                    $entry=@($Catalog.controlled_selection.subcases.PSObject.Properties | Where-Object {$_.Name -ceq $id})
                    if ($entry.Count -and $entry[0].Value.PSObject.Properties[$parent]) { $narrow=@($entry[0].Value.$parent) }
                }
                if (-not $narrow.Count -and $parent -ceq 'TestSRResultFile' -and $Catalog.controlled_selection.PSObject.Properties['result_file_cases']) {
                    $entry=@($Catalog.controlled_selection.result_file_cases.PSObject.Properties | Where-Object {$_.Name -ceq $id})
                    if ($entry.Count) { $narrow=@($entry[0].Value) }
                }
            }
            if (-not $narrow.Count) { $narrow=@($definitions[$parent]) }
            foreach ($caseID in $narrow) { $null=$wanted.Add($caseID) }
        }
        $plan[$parent]=@($definitions[$parent] | Where-Object {$wanted.Contains($_)})
    }
    $legacy=@(if($plan.ContainsKey('TestSRResultFile')){$plan['TestSRResultFile']})
    @{kind=$kind;ids=@($ids);tests=$tests;cases=$plan;result_file_cases=$legacy;pattern=('^('+(($tests | ForEach-Object {[regex]::Escape($_)}) -join '|')+')$')}
}

function Get-SRControlledTimingReport($Catalog, [string]$Directory, [string[]]$ExpectedTests=@(), [string[]]$ExpectedResultFileCases=@(), [Collections.IDictionary]$ExpectedCases=@{}) {
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
    $subtests=Read-SRControlledSubtests $directoryPath
    foreach ($errorText in $subtests.errors) { $errors.Add($errorText) }
    $casePlan=@{}
    foreach ($parent in $ExpectedCases.Keys) { $casePlan[$parent]=@($ExpectedCases[$parent]) }
    if ($ExpectedResultFileCases.Count) {
        if ($casePlan.ContainsKey('TestSRResultFile')) { throw 'Duplicate ResultFile expected case plan' }
        $casePlan['TestSRResultFile']=$ExpectedResultFileCases
    }
    $coverage=@{}
    $definitions=Get-SRControlledCaseDefinitions
    foreach ($parent in $casePlan.Keys) { Assert-SRControlledCaseSelection $parent @($casePlan[$parent]) }
    foreach ($parent in $definitions.Keys) {
        $coverage[$parent]=@{status='not_required';expected_cases=@()}
        if ($casePlan.ContainsKey($parent)) {
            try { $coverage[$parent]=Get-SRControlledCaseEvidence $directoryPath $parent @($casePlan[$parent]) }
            catch { $errors.Add($_.Exception.Message); $coverage[$parent]=@{status='invalid';expected_cases=@($casePlan[$parent]);error=$_.Exception.Message} }
        } elseif ($records.ContainsKey($parent)) { $coverage[$parent]=@{status='unverified_import';expected_cases=@()} }
    }
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
      case_coverage=$coverage;result_file_coverage=$coverage['TestSRResultFile'];subtests=@($subtests.records);
      tests=@($tests | Sort-Object test);groups=$groups;slowest=@($measured | Sort-Object @{Expression='duration_seconds';Descending=$true},test | Select-Object -First 10)}
}
