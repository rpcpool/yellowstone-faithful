DEFAULT:compile

IPLD_SCHEMA_PATH := ledger.ipldsch

compile:
	@echo "\nCompiling faithful-cli binary for current platform ..."
	go build -o ./bin/faithful-cli .
compile-all: compile-linux compile-mac
compile-linux:
	@echo "\nCompiling faithful-cli binary for linux amd64 ..."
	GOOS=linux GOARCH=amd64 go build -o ./bin/linux/amd64/faithful-cli_linux_amd64 .
compile-mac:
	@echo "\nCompiling faithful-cli binary for mac amd64 ..."
	GOOS=darwin GOARCH=amd64 go build -o ./bin/darwin/amd64/faithful-cli_darwin_amd64 .

	@echo "\nCompiling faithful-cli binary for mac arm64 ..."
	GOOS=darwin GOARCH=arm64 go build -o ./bin/darwin/arm64/faithful-cli_darwin_arm64 .
compile-windows:
	@echo "\nCompiling faithful-cli binary for windows amd64 ..."
	GOOS=windows GOARCH=amd64 go build -o ./bin/windows-amd64/faithful-cli_windows_amd64
test:
	go test -v ./...
bindcode: install-deps
	ipld schema codegen \
		--generator=go-bindnode \
		--package=ipldbindcode \
		--output=./ipld/ipldbindcode \
		$(IPLD_SCHEMA_PATH)
gengo: install-deps
	ipld schema codegen \
		--generator=go-gengo \
		--package=ipldsch \
		--output=./ipld/ipldsch \
		$(IPLD_SCHEMA_PATH)
install-deps:
	go install github.com/ipld/go-ipldtool/cmd/ipld@latest
install-protoc:
	@echo "Installing protoc..."
	@mkdir -p $$(pwd)/third_party/protoc
	@echo "Getting the latest release of protoc from github.com/protocolbuffers/protobuf..."
	@cd $$(pwd)/third_party/protoc && \
		wget https://github.com/protocolbuffers/protobuf/releases/download/v23.1/protoc-23.1-linux-x86_64.zip
	@echo "Unzipping protoc..."
	@cd $$(pwd)/third_party/protoc && \
		unzip protoc-23.1-linux-x86_64.zip
	@echo "Installing protoc-gen-go..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
gen-proto: install-protoc
	@echo "Generating proto files..."
	$$(pwd)/third_party/protoc/bin/protoc \
		--experimental_allow_proto3_optional \
		--go_out=paths=source_relative:$$(pwd)/third_party/solana_proto/confirmed_block \
		-I=$$(pwd)/third_party/solana_proto/confirmed_block/ \
		$$(pwd)/third_party/solana_proto/confirmed_block/confirmed_block.proto
