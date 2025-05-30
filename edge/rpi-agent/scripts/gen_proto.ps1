$ErrorActionPreference = "Stop"

# Set paths
$protoDir = Join-Path $PSScriptRoot "..\..\..\proto"
$outputDir = Join-Path $PSScriptRoot "..\..\internal\pb"

# Create output directory if it doesn't exist
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

# Generate Go code
$protocArgs = @(
    "--go_out=$outputDir",
    "--go_opt=paths=source_relative",
    "--go-grpc_out=$outputDir",
    "--go-grpc_opt=paths=source_relative",
    "-I=$protoDir",
    (Join-Path $protoDir "axcp.proto")
)

& protoc @protocArgs

if ($LASTEXITCODE -ne 0) {
    Write-Error "Failed to generate protobuf code"
    exit 1
}

Write-Host "Protobuf code generated successfully in $outputDir"
