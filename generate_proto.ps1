$ErrorActionPreference = "Stop"

$protoc = "protoc"
$protoDir = "proto"
$protoFile = "proto/axcp.proto"
$goOutDir = "sdk/go/internal/pb"

Write-Host "Generating Go protobuf stubs..."
New-Item -ItemType Directory -Force -Path $goOutDir | Out-Null
& $protoc -I $protoDir `
    --go_out=$goOutDir `
    --go_opt=paths=source_relative `
    $protoFile

Write-Host "Generating Python protobuf stubs..."
python -m grpc_tools.protoc -I=$protoDir --python_out=$protoDir $protoFile

Write-Host "Protobuf files generated successfully."
