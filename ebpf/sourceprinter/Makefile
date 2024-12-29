A = $(shell uname -m)
ARCH = $(shell uname -m | sed 's/x86_64/x86/' | sed 's/aarch64/arm64/')

all:
	clang -target bpf -c sourceip.bpf.c -g -O2 -Wall -D __TARGET_ARCH_$(ARCH) -I/usr/include/$(A)-linux-gnu
	llvm-strip -g sourceip.bpf.o

clean:
	rm sourceip.bpf.o

