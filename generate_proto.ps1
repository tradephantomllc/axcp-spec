$protoc = "protoc"
$proto_file = "proto/axcp.proto"
$go_out = "--go_out=paths=source_relative:."
$go_opt = "--go_opt=module=github.com/tradephantom/axcp-spec"

# Clean up existing files
Remove-Item -Force -Recurse -ErrorAction SilentlyContinue proto/*.pb.go
Remove-Item -Force -Recurse -ErrorAction SilentlyContinue edge/rpi-agent/internal/pb/*.pb.go

# Generate Go code
Write-Host "Generating protobuf files..."
& $protoc --go_out=paths=source_relative:. --go_opt=Mproto/axcp.proto=github.com/tradephantom/axcp-spec/edge/rpi-agent/internal/pb $proto_file

# Move generated files to the correct location
$generated_file = "proto/axcp.pb.go"
if (Test-Path $generated_file) {
    $target_dir = "edge/rpi-agent/internal/pb"
    New-Item -ItemType Directory -Force -Path $target_dir | Out-Null
    Move-Item -Force $generated_file $target_dir
    Write-Host "Protobuf files generated successfully in $target_dir"
} else {
    Write-Error "Failed to generate protobuf files"
}
