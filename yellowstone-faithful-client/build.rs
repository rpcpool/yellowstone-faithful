fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Set the protoc path
    std::env::set_var("PROTOC", protobuf_src::protoc());

    // Configure protobuf compilation
    tonic_build::configure()
        .build_server(false) // We only need the client
        .build_client(true)
        .compile(
            &["../old-faithful-proto/proto/old-faithful.proto"],
            &["../old-faithful-proto/proto"],
        )?;

    // Ensure rebuild on proto file changes
    println!("cargo:rerun-if-changed=../old-faithful-proto/proto/old-faithful.proto");

    Ok(())
}
