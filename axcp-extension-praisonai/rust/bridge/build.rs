fn main() {
    prost_build::compile_protos(&["../../proto/axcp.proto"], &["../../proto"]).unwrap();
}
