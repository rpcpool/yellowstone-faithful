DEFAULT := plugin

# Path: Makefile
plugin:
	@echo "Building plugin..."
	@cd demo-plugin && cargo build --release --lib --target=x86_64-unknown-linux-gnu --target-dir=target
faithful:
	@echo "Building rust faithful..."
	@cargo build --release --target-dir=target
	