all:
	sudo bpftool btf dump file /sys/kernel/btf/vmlinux format c > vmlinux.h
	clang -target bpf -c openatsnoop.bpf.c -g -O2
	llvm-strip -g openatsnoop.bpf.o
	sudo bpftool gen skeleton openatsnoop.bpf.o > openatsnoop.skel.h
	clang openatsnoop.c -L/usr/lib64/ -lbpf -lelf -Wl, -rpath /usr/lib64/

clean:
	rm openatsnoop.bpf.o
	rm a.out
