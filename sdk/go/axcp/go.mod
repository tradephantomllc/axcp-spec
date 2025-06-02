module github.com/tradephantom/axcp-spec/sdk/go/axcp

go 1.22

require (
	github.com/stretchr/testify v1.9.0
	github.com/tradephantom/axcp-spec/sdk/go/internal/pb v0.0.0
	google.golang.org/protobuf v1.33.0
)

replace github.com/tradephantom/axcp-spec/sdk/go/internal/pb => ../internal/pb

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
