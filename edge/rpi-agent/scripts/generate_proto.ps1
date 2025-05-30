$ErrorActionPreference = "Stop"

# Create output directory if it doesn't exist
$outputDir = "$PSScriptRoot/../../internal/pb"
New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

# Path to the proto file
$protoFile = "$PSScriptRoot/../../../proto/axcp.proto"

# Generate Go code
protoc --go_out=$outputDir --go_opt=paths=source_relative `
       --go-grpc_out=$outputDir --go-grpc_opt=paths=source_relative `
       -I $PSScriptRoot/../../../proto `
       $protoFile

Write-Host "Protobuf code generated successfully in $outputDir"
