export GOOS := linux
export GOARCH := arm64
export CGO_ENABLED := 0
export GO_LDFLAGS := "-s -w -buildid="

build-GetObjectFunction:
	go build -o $(ARTIFACTS_DIR)/bootstrap -trimpath -ldflags=$(GO_LDFLAGS)

build-HeadObjectFunction:
	go build -o $(ARTIFACTS_DIR)/bootstrap -trimpath -ldflags=$(GO_LDFLAGS)
