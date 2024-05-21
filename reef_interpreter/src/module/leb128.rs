//! A leb128 reader for u32

use std::{
    io::{Error, Read, Result},
    mem::size_of,
};

const CONTINUE_BIT: u8 = 1 << 7;
const SIGN_BIT: u8 = 1 << 6;

/// Extension for anything that implements `Read` to easily read LEB128 numbers.
pub trait LEB128Ext {
    /// Read unsigned u32 encoded as LEB128.
    fn read_u32_leb(&mut self) -> Result<u32>;

    /// Read signed i32 encoded as LEB128.
    fn read_i32_leb(&mut self) -> Result<i32>;

    /// Read signed i64 encoded as LEB128.
    fn read_i64_leb(&mut self) -> Result<i64>;
}

impl<R: Read> LEB128Ext for R {
    fn read_u32_leb(&mut self) -> Result<u32> {
        let mut result = 0;
        let mut shift = 0;

        let mut buf = [0];
        loop {
            self.read_exact(&mut buf)?;
            result |= ((buf[0] & !CONTINUE_BIT) as u32) << shift;

            if buf[0] & CONTINUE_BIT == 0 {
                return Ok(result);
            }

            if shift == 7 * size_of::<u32>() {
                return Err(Error::other("LEB overflow"));
            }
            shift += 7;
        }
    }

    fn read_i32_leb(&mut self) -> Result<i32> {
        let mut result = 0;
        let mut shift = 0;

        let mut buf = [0];
        loop {
            self.read_exact(&mut buf)?;

            result |= ((buf[0] & !CONTINUE_BIT) as i32) << shift;

            if buf[0] & CONTINUE_BIT == 0 {
                break;
            }

            if shift == 7 * size_of::<i32>() {
                return Err(Error::other("LEB overflow"));
            }
            shift += 7;
        }

        if (SIGN_BIT & buf[0]) == SIGN_BIT {
            // Sign extend the result.
            result |= !0 << (shift + 7);
        }

        Ok(result)
    }

    fn read_i64_leb(&mut self) -> Result<i64> {
        let mut result = 0;
        let mut shift = 0;

        let mut buf = [0];
        loop {
            self.read_exact(&mut buf)?;

            result |= ((buf[0] & !CONTINUE_BIT) as i64) << shift;

            if buf[0] & CONTINUE_BIT == 0 {
                break;
            }

            if shift == 7 * size_of::<i64>() {
                return Err(Error::other("LEB overflow"));
            }
            shift += 7;
        }

        if (SIGN_BIT & buf[0]) == SIGN_BIT {
            // Sign extend the result.
            result |= !0 << (shift + 7);
        }

        Ok(result)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Cursor;

    #[test]
    fn read_u_zero() {
        let mut buf = Cursor::new([0]);
        assert_eq!(buf.read_u32_leb().unwrap(), 0);
    }

    #[test]
    fn read_u_num() {
        let mut buf = Cursor::new([0xA4, 0x03]);
        assert_eq!(buf.read_u32_leb().unwrap(), 420);
    }

    #[test]
    fn read_i_zero() {
        let mut buf = Cursor::new([0]);
        assert_eq!(buf.read_i32_leb().unwrap(), 0);
    }

    #[test]
    fn read_i_num() {
        let mut buf = Cursor::new([0xA4, 0x03]);
        assert_eq!(buf.read_i32_leb().unwrap(), 420);
    }

    #[test]
    fn read_i_num_neg() {
        let mut buf = Cursor::new([0xDC, 0x7C]);
        assert_eq!(buf.read_i32_leb().unwrap(), -420);
    }

    #[test]
    fn read_i_num_large() {
        let mut buf = Cursor::new([0xCE, 0xC2, 0xF1, 0x05]);
        assert_eq!(buf.read_i32_leb().unwrap(), 12345678);
    }

    #[test]
    fn read_i_num_large_neg() {
        let mut buf = Cursor::new([0xB2, 0xBD, 0x8E, 0x7A]);
        assert_eq!(buf.read_i32_leb().unwrap(), -12345678);
    }
}
