use {
    crate::{byte_order, type_size},
    std::error::Error as StdError,
};

// declare error type
pub enum Error {
    ShortBuffer { msg: String },
    InvalidValue { msg: String },
}

impl StdError for Error {}

impl std::fmt::Display for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            Error::ShortBuffer { msg } => write!(f, "short buffer: {}", msg),
            Error::InvalidValue { msg } => write!(f, "invalid value: {}", msg),
        }
    }
}

impl std::fmt::Debug for Error {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match self {
            Error::ShortBuffer { msg } => write!(f, "short buffer: {}", msg),
            Error::InvalidValue { msg } => write!(f, "invalid value: {}", msg),
        }
    }
}

pub struct Decoder {
    data: Vec<u8>,
    pos: usize,
}

#[allow(dead_code)]
impl Decoder {
    pub fn new(data: Vec<u8>) -> Decoder {
        Decoder { data, pos: 0 }
    }

    pub fn reset(&mut self, data: Vec<u8>) {
        self.data = data;
        self.pos = 0;
    }

    pub fn read_byte(&mut self) -> Result<u8, Error> {
        if self.pos + type_size::BYTE > self.data.len() {
            return Err(Error::ShortBuffer {
                msg: format!(
                    "required {} bytes, but only {} bytes available",
                    type_size::BYTE,
                    self.remaining()
                ),
            });
        }
        let b = self.data[self.pos];
        self.pos += type_size::BYTE;
        Ok(b)
    }

    fn read_n_bytes(&mut self, n: usize) -> Result<Vec<u8>, Error> {
        if n == 0 {
            return Ok(Vec::new());
        }
        if n > 0x7FFF_FFFF {
            return Err(Error::ShortBuffer {
                msg: format!("n not valid: {}", n),
            });
        }
        if self.pos + n > self.data.len() {
            return Err(Error::ShortBuffer {
                msg: format!(
                    "required {} bytes, but only {} bytes available",
                    n,
                    self.remaining()
                ),
            });
        }
        let out = self.data[self.pos..self.pos + n].to_vec();
        self.pos += n;
        Ok(out)
    }

    pub fn remaining(&self) -> usize {
        self.data.len() - self.pos
    }

    pub fn read(&mut self, buf: &mut [u8]) -> Result<usize, Error> {
        if self.pos + buf.len() > self.data.len() {
            return Err(Error::ShortBuffer {
                msg: format!(
                    "not enough data: {} bytes missing",
                    self.pos + buf.len() - self.data.len()
                ),
            });
        }
        let num_copied = buf.len();
        buf.copy_from_slice(&self.data[self.pos..self.pos + buf.len()]);
        if num_copied != buf.len() {
            return Err(Error::ShortBuffer {
                msg: format!(
                    "expected to read {} bytes, but read only {} bytes",
                    buf.len(),
                    num_copied
                ),
            });
        }
        self.pos += num_copied;
        Ok(num_copied)
    }

    pub fn read_bytes(&mut self, n: usize) -> Result<Vec<u8>, Error> {
        self.read_n_bytes(n)
    }

    pub fn read_option(&mut self) -> Result<bool, Error> {
        let b = self.read_byte()?;
        let out = b != 0;
        Ok(out)
    }

    pub fn read_u8(&mut self) -> Result<u8, Error> {
        let out = self.read_byte()?;
        Ok(out)
    }

    pub fn read_u16(&mut self) -> Result<u16, Error> {
        if self.remaining() < type_size::UINT16 {
            return Err(Error::InvalidValue {
                msg: format!(
                    "uint16 requires [{}] bytes, remaining [{}]",
                    type_size::UINT16,
                    self.remaining()
                ),
            });
        }
        let buf = self.read_bytes(type_size::UINT16)?;
        let buf: [u8; 2] = buf.try_into().unwrap();
        let out = u16::from_le_bytes(buf);
        Ok(out)
    }

    pub fn read_u32(&mut self, order: byte_order::ByteOrder) -> Result<u32, Error> {
        if self.remaining() < type_size::UINT32 {
            return Err(Error::InvalidValue {
                msg: format!(
                    "uint32 requires [{}] bytes, remaining [{}]",
                    type_size::UINT32,
                    self.remaining()
                ),
            });
        }
        let buf = self.read_bytes(type_size::UINT32)?;
        let buf: [u8; 4] = buf.try_into().unwrap();
        let out = match order {
            byte_order::ByteOrder::LittleEndian => u32::from_le_bytes(buf),
            byte_order::ByteOrder::BigEndian => u32::from_be_bytes(buf),
        };
        Ok(out)
    }

    pub fn set_position(&mut self, idx: usize) -> Result<(), Error> {
        if idx < self.data.len() {
            self.pos = idx;
            Ok(())
        } else {
            Err(Error::InvalidValue {
                msg: format!(
                    "request to set position to {} outsize of buffer (buffer size {})",
                    idx,
                    self.data.len()
                ),
            })
        }
    }

    pub fn position(&self) -> usize {
        self.pos
    }

    pub fn len(&self) -> usize {
        self.data.len()
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    pub fn has_remaining(&self) -> bool {
        self.remaining() > 0
    }
}

// declare TypeID as a [u8; 8]
pub type TypeID = [u8; 8];

// use extension trait to add a method to the TypeID type
pub trait TypeIDFromBytes {
    fn from_bytes(bytes: Vec<u8>) -> TypeID;
}

impl TypeIDFromBytes for TypeID {
    fn from_bytes(bytes: Vec<u8>) -> TypeID {
        let mut type_id = [0u8; 8];
        type_id.copy_from_slice(&bytes);
        type_id
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_decoder_uint8() {
        let buf = vec![0x63, 0x64];

        let mut d = Decoder::new(buf);

        let n = d.read_u8().unwrap();
        assert_eq!(99, n);
        assert_eq!(1, d.remaining());

        let n = d.read_u8().unwrap();
        assert_eq!(100, n);
        assert_eq!(0, d.remaining());
    }

    #[test]
    fn test_decoder_u16() {
        // little endian
        let mut buf = vec![];
        buf.extend_from_slice(18360u16.to_le_bytes().as_ref());
        buf.extend_from_slice(28917u16.to_le_bytes().as_ref());
        buf.extend_from_slice(1023u16.to_le_bytes().as_ref());
        buf.extend_from_slice(0u16.to_le_bytes().as_ref());
        buf.extend_from_slice(33u16.to_le_bytes().as_ref());

        let mut d = Decoder::new(buf);

        let n = d.read_u16().unwrap();
        assert_eq!(18360, n);
        assert_eq!(8, d.remaining());

        let n = d.read_u16().unwrap();
        assert_eq!(28917, n);
        assert_eq!(6, d.remaining());

        let n = d.read_u16().unwrap();
        assert_eq!(1023, n);
        assert_eq!(4, d.remaining());

        let n = d.read_u16().unwrap();
        assert_eq!(0, n);
        assert_eq!(2, d.remaining());

        let n = d.read_u16().unwrap();
        assert_eq!(33, n);
        assert_eq!(0, d.remaining());
    }

    #[test]
    fn test_decoder_byte() {
        let buf = vec![0x00, 0x01];

        let mut d = Decoder::new(buf);

        let n = d.read_byte().unwrap();
        assert_eq!(0, n);
        assert_eq!(1, d.remaining());

        let n = d.read_byte().unwrap();
        assert_eq!(1, n);
        assert_eq!(0, d.remaining());
    }

    #[test]
    fn test_decoder_read_bytes() {
        let mut buf = vec![];
        buf.extend_from_slice(&[0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff]);
        let mut decoder = Decoder::new(buf);
        let b = decoder.read_bytes(1).unwrap();
        assert_eq!(vec![0xff], b);
        assert_eq!(7, decoder.remaining());

        let b = decoder.read_bytes(2).unwrap();
        assert_eq!(vec![0xff, 0xff], b);
        assert_eq!(5, decoder.remaining());

        decoder.read_bytes(6).unwrap_err();

        let b = decoder.read_bytes(5).unwrap();
        assert_eq!(vec![0xff, 0xff, 0xff, 0xff, 0xff], b);
        assert_eq!(0, decoder.remaining());
    }

    #[test]
    fn test_read_n_bytes() {
        let mut b1 = vec![];
        b1.extend_from_slice(&[123, 99, 88, 77, 66, 55, 44, 33, 22, 11]);
        let mut b2 = vec![];
        b2.extend_from_slice(&[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
        let mut buf = vec![];
        buf.extend_from_slice(&b1);
        buf.extend_from_slice(&b2);
        let mut decoder = Decoder::new(buf);

        let got = decoder.read_n_bytes(10).unwrap();
        assert_eq!(b1, got);

        let got = decoder.read_n_bytes(10).unwrap();
        assert_eq!(b2, got);
    }

    #[test]
    fn test_read_n_bytes_error() {
        let mut b1 = vec![];
        b1.extend_from_slice(&[123, 99, 88, 77, 66, 55, 44, 33, 22, 11]);
        let mut b2 = vec![];
        b2.extend_from_slice(&[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
        let mut buf = vec![];
        buf.extend_from_slice(&b1);
        buf.extend_from_slice(&b2);
        let mut decoder = Decoder::new(buf);

        let res = decoder.read_n_bytes(9999);
        assert!(res.is_err());
    }

    #[test]
    fn test_read_bytes() {
        let mut b1 = vec![];
        b1.extend_from_slice(&[123, 99, 88, 77, 66, 55, 44, 33, 22, 11]);
        let mut b2 = vec![];
        b2.extend_from_slice(&[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
        let mut buf = vec![];
        buf.extend_from_slice(&b1);
        buf.extend_from_slice(&b2);
        let mut decoder = Decoder::new(buf);

        let got = decoder.read_bytes(10).unwrap();
        assert_eq!(b1, got);

        let got = decoder.read_bytes(10).unwrap();
        assert_eq!(b2, got);
    }

    #[test]
    fn test_read() {
        let mut b1 = vec![];
        b1.extend_from_slice(&[123, 99, 88, 77, 66, 55, 44, 33, 22, 11]);
        let mut b2 = vec![];
        b2.extend_from_slice(&[1, 2, 3, 4, 5, 6, 7, 8, 9, 10]);
        let mut buf = vec![];
        buf.extend_from_slice(&b1);
        buf.extend_from_slice(&b2);
        let mut decoder = Decoder::new(buf);

        {
            let mut got = vec![];
            got.extend_from_slice(&[0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
            let num = decoder.read(&mut got).unwrap();
            assert_eq!(b1, got);
            assert_eq!(10, num);
        }

        {
            let mut got = vec![];
            got.extend_from_slice(&[0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
            let num = decoder.read(&mut got).unwrap();
            assert_eq!(b2, got);
            assert_eq!(10, num);
        }
        {
            let mut got = vec![];
            got.extend_from_slice(&[0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0]);
            let res = decoder.read(&mut got);
            assert!(res.is_err());
        }
        {
            let mut got = vec![];
            let num = decoder.read(&mut got).unwrap();
            assert_eq!(0, num);
            assert_eq!(vec![] as Vec<u8>, got);
        }
    }

    #[test]
    fn test_decoder_uint32() {
        // little endian
        let buf = vec![0x28, 0x72, 0x75, 0x10, 0x4f, 0x9f, 0x03, 0x00];

        let mut d = Decoder::new(buf);

        let n = d.read_u32(byte_order::ByteOrder::LittleEndian).unwrap();
        assert_eq!(276132392, n);
        assert_eq!(4, d.remaining());

        let n = d.read_u32(byte_order::ByteOrder::LittleEndian).unwrap();
        assert_eq!(237391, n);
        assert_eq!(0, d.remaining());

        // big endian
        let buf = vec![0x10, 0x75, 0x72, 0x28, 0x00, 0x03, 0x9f, 0x4f];

        let mut d = Decoder::new(buf);

        let n = d.read_u32(byte_order::ByteOrder::BigEndian).unwrap();
        assert_eq!(276132392, n);
        assert_eq!(4, d.remaining());

        let n = d.read_u32(byte_order::ByteOrder::BigEndian).unwrap();
        assert_eq!(237391, n);
        assert_eq!(0, d.remaining());
    }
}
