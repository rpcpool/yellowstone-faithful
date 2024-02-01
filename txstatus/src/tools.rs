fn read_uint32(bytes: &[u8]) -> u32 {
    let mut result = 0;
    for i in 0..4 {
        result |= (bytes[i] as u32) << (i * 8);
    }
    result
}

fn read_n_bytes(bytes: &[u8], n: usize) -> Vec<u8> {
    let mut result = Vec::new();
    for i in 0..n {
        result.push(bytes[i]);
    }
    result
}
