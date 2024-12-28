fn main() {
    lc3_vm::VM::new(std::env::args().collect()).run();
}
