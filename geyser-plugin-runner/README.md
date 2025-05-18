# yellowstone-faithful-rust

This is a Rust implementation of the [Yellowstone Faithful](https://github.com/rpcpool/yellowstone-faithful) project.

Right now, it only supports reading a CAR file dumping the contents to a Geyser plugin.

## Usage

```bash
# Modify and build the geyser plugin
cd demo-plugin
cargo build --release --lib --target=x86_64-unknown-linux-gnu --target-dir=target

# Modify the [plugin config json file](demo-plugin/src/plugin-config.json)
# The important part is the `libpath` field, which should point to the built plugin (absolute path).
nano demo-plugin/src/plugin-config.json

# Run the demo
cargo run /path/to/epoch-NNN.car demo-plugin/src/plugin-config.json

or 

cargo run https://files.old-faithful.net/NNN/epoch-NNN.car demo-plugin/src/plugin-config.json
