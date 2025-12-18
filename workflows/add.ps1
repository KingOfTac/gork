param([int]$number)
if ($number -eq 0 -and $env:INPUT_VALUE) {
    $number = [int]$env:INPUT_VALUE
}
Write-Host "Starting PowerShell script..."
Write-Host "Input: $number"
$result = $number + 10
Write-Host "Result: $result"
Write-Host "OUTPUT_RESULT:$result"
Write-Host "PowerShell script completed."