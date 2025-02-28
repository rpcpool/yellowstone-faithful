DEFAULT:compile

IPLD_SCHEMA_PATH := ledger.ipldsch
BASE_LD_FLAGS := -X main.GitCommit=`git rev-parse HEAD` -X main.GitTag=`git symbolic-ref -q --short HEAD || git describe --tags --exact-match || git rev-parse HEAD`

ROOT_DIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))

build-rust-wrapper:
	rm -rf jsonparsed/lib
	cd jsonparsed && cargo build --release --lib --target=x86_64-unknown-linux-gnu --target-dir=target
	cbindgen ./jsonparsed -o jsonparsed/lib/transaction_status.h --lang c
	echo "build-rust-wrapper done"
jsonParsed-linux: build-rust-wrapper
	# build faithful-cli with jsonParsed format support via ffi (rust)
	rm -rf ./bin/faithful-cli_jsonParsed
	# static linking:
	cp jsonparsed/target/x86_64-unknown-linux-gnu/release/libdemo_transaction_status_ffi.a ./jsonparsed/lib/libsolana_transaction_status_wrapper.a
	LD_FLAGS="$(BASE_LD_FLAGS) -extldflags -static"
	go build -ldflags=$(LD_FLAGS) -tags ffi -o ./bin/faithful-cli_jsonParsed .
	echo "SUCCESS: built old-faithful with jsonParsed format support via ffi (rust)"
compile:
	@echo "\nCompiling faithful-cli binary for current platform ..."
	go build -ldflags="$(BASE_LD_FLAGS)" -o ./bin/faithful-cli .
	chmod +x ./bin/faithful-cli
compile-all: compile-linux compile-mac compile-windows
compile-linux:
	@echo "\nCompiling faithful-cli binary for linux amd64 ..."
	GOOS=linux GOARCH=amd64 go build -ldflags="$(BASE_LD_FLAGS)" -o ./bin/linux/amd64/faithful-cli_linux_amd64 .
	chmod +x ./bin/linux/amd64/faithful-cli_linux_amd64
compile-mac:
	@echo "\nCompiling faithful-cli binary for mac amd64 ..."
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(BASE_LD_FLAGS)" -o ./bin/darwin/amd64/faithful-cli_darwin_amd64 .

	@echo "\nCompiling faithful-cli binary for mac arm64 ..."
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(BASE_LD_FLAGS)" -o ./bin/darwin/arm64/faithful-cli_darwin_arm64 .
compile-windows:
	@echo "\nCompiling faithful-cli binary for windows amd64 ..."
	GOOS=windows GOARCH=amd64 go build -ldflags="$(BASE_LD_FLAGS)" -o ./bin/windows/amd64/faithful-cli_windows_amd64.exe .
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
		wget https://github.com/protocolbuffers/protobuf/releases/download/v27.1/protoc-27.1-linux-x86_64.zip
	@echo "Unzipping protoc..."
	@cd $$(pwd)/third_party/protoc && \
		unzip protoc-27.1-linux-x86_64.zip
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
	$$(pwd)/third_party/protoc/bin/protoc \
		--experimental_allow_proto3_optional \
		--go_out=paths=source_relative:$$(pwd)/third_party/solana_proto/transaction_by_addr \
		-I=$$(pwd)/third_party/solana_proto/transaction_by_addr/ \
		$$(pwd)/third_party/solana_proto/transaction_by_addr/transaction_by_addr.proto
gen-old-faithful-proto: install-protoc
	@mkdir -p $$(pwd)/old-faithful-proto/old-faithful-grpc
	@echo "Generating golang protobuf for old-faithful..."
	# the proto file is in old-faithful-proto/proto ; generate go code in old-faithful-proto/faithful-grpc
	$$(pwd)/third_party/protoc/bin/protoc \
		--experimental_allow_proto3_optional \
		--go_out=paths=source_relative:$$(pwd)/old-faithful-proto/old-faithful-grpc \
		--go-grpc_out=paths=source_relative:$$(pwd)/old-faithful-proto/old-faithful-grpc \
		-I=$$(pwd)/old-faithful-proto/proto/ \
		$$(pwd)/old-faithful-proto/proto/old-faithful.proto
gen-old-faithful-proto-system:
	@mkdir -p $$(pwd)/old-faithful-proto/old-faithful-grpc
	@echo "Generating golang protobuf for old-faithful..."
	# the proto file is in old-faithful-proto/proto ; generate go code in old-faithful-proto/faithful-grpc
	protoc \
		--experimental_allow_proto3_optional \
		--go_out=paths=source_relative:$$(pwd)/old-faithful-proto/old-faithful-grpc \
		--go-grpc_out=paths=source_relative:$$(pwd)/old-faithful-proto/old-faithful-grpc \
		-I=$$(pwd)/old-faithful-proto/proto/ \
		$$(pwd)/old-faithful-proto/proto/old-faithful.proto
