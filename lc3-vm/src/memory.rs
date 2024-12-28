//! LC3 Virtual memory
//!
//! Provides a constant 128KB memory abstraction for the LC3 architecture
use std::io::{self, Read};

/// LC3 memory registers for keyboard status/input.
#[repr(u16)]
pub enum MemRegister {
    KBSR = 0xFE00, // keyboard status
    KBDR = 0xFE02, // keyboard data
}

/// Virtual memory with a total of 128Kb size.
pub struct Memory([u16; 1 << 16]);

impl Memory {
    /// Returns a zero intialized virtual memory.
    ///
    /// # Examples
    ///
    /// ```
    /// use lc3_vm::memory::Memory;
    ///
    /// let _ = Memory::new();
    /// ```
    pub fn new() -> Self {
        Memory([0 as u16; 1 << 16])
    }

    /// Returns a immutable reference to the underlying memory.
    ///
    /// # Examples
    ///
    /// ```
    /// use lc3_vm::memory::Memory;
    ///
    /// let m = Memory::new();
    /// let r = m.raw();
    ///
    /// assert_eq!(r.len(), 1<<16);
    /// ```
    pub fn raw(&self) -> &[u16] {
        &self.0
    }

    /// Returns a mmutable reference to the underlying memory.
    ///
    /// # Examples
    ///
    /// ```
    /// use lc3_vm::memory::Memory;
    ///
    /// let mut m = Memory::new();
    /// let r = m.raw_mut();
    /// r[0] = 0x1;
    ///
    /// assert_eq!(m.raw()[0], 0x1);
    /// ```
    pub fn raw_mut(&mut self) -> &mut [u16] {
        &mut self.0
    }

    /// Writes the value to the given address.
    ///
    /// # Examples
    ///
    /// ```
    /// use lc3_vm::memory::Memory;
    ///
    /// let mut m = Memory::new();
    /// m.memory_write(0x1, 0x5);
    ///
    /// assert_eq!(m.memory_read(0x1), 0x5);
    /// ```
    pub fn memory_write(&mut self, addr: u16, val: u16) {
        self.0[addr as usize] = val;
    }

    /// Read the value from the given address.
    ///
    /// # Examples
    ///
    /// ```
    /// use lc3_vm::memory::Memory;
    ///
    /// let mut m = Memory::new();
    /// m.memory_write(0x1, 0x5);
    ///
    /// assert_eq!(m.memory_read(0x1), 0x5);
    /// ```
    pub fn memory_read(&mut self, addr: u16) -> u16 {
        if addr == MemRegister::KBSR as u16 {
            let mut buffer = [0 as u8; 1];
            io::stdin()
                .read_exact(&mut buffer)
                .expect("failed to read stdin");

            if buffer[0] != 0 {
                self.0[MemRegister::KBSR as usize] = 1 << 15;
                self.0[MemRegister::KBDR as usize] = buffer[0] as u16;
            } else {
                self.0[MemRegister::KBSR as usize] = 0;
            }
        }

        self.0[addr as usize]
    }
}
