# openatsnoop

Is an eBPF program that listens to all openat syscalls from every process.

Example output after running:

```
libbpf: loading object 'openatsnoop_bpf' from buffer
libbpf: elf: section(3) tracepoint/syscalls/sys_enter_openat, size 168, link 0, flags 6, type=1
libbpf: sec 'tracepoint/syscalls/sys_enter_openat': found program 'openat' at insn offset 0 (0 bytes), code size 21 insns (168 bytes)
libbpf: elf: section(4) .reltracepoint/syscalls/sys_enter_openat, size 16, link 12, flags 40, type=9
libbpf: elf: section(5) license, size 4, link 0, flags 3, type=1
libbpf: license of openatsnoop_bpf is GPL
libbpf: elf: section(6) .maps, size 16, link 0, flags 3, type=1
libbpf: elf: section(7) .BTF, size 1218, link 0, flags 0, type=1
libbpf: elf: section(9) .BTF.ext, size 236, link 0, flags 0, type=1
libbpf: elf: section(12) .symtab, size 144, link 1, flags 0, type=2
libbpf: looking for externs among 6 symbols...
libbpf: collected 0 externs total
libbpf: map 'buffer': at sec_idx 6, offset 0.
libbpf: map 'buffer': found type = 27.
libbpf: map 'buffer': found max_entries = 1024.
libbpf: sec '.reltracepoint/syscalls/sys_enter_openat': collecting relocation for section(3) 'tracepoint/syscalls/sys_enter_openat'
libbpf: sec '.reltracepoint/syscalls/sys_enter_openat': relo #0: insn #1 against 'buffer'
libbpf: prog 'openat': found map 0 (buffer, sec 6, off 0) for insn #1
libbpf: object 'openatsnoop_bpf': failed (-22) to create BPF token from '/sys/fs/bpf', skipping optional step...
libbpf: loaded kernel BTF from '/sys/kernel/btf/vmlinux'
libbpf: sec 'tracepoint/syscalls/sys_enter_openat': found 1 CO-RE relocations
libbpf: CO-RE relocating [10] struct trace_event_raw_sys_enter: found target candidate [2380] struct trace_event_raw_sys_enter in [vmlinux]
libbpf: prog 'openat': relo #0: <byte_off> [10] struct trace_event_raw_sys_enter.args[1] (0:2:1 @ offset 24)
libbpf: prog 'openat': relo #0: matching candidate #0 <byte_off> [2380] struct trace_event_raw_sys_enter.args[1] (0:2:1 @ offset 24)
libbpf: prog 'openat': relo #0: patched insn #11 (LDX/ST/STX) off 24 -> 24
libbpf: map 'buffer': created successfully, fd=3
systemd-oomd openat: /proc/meminfo
irqbalance openat: /proc/interrupts
irqbalance openat: /proc/stat
irqbalance openat: /proc/irq/64/smp_affinity
irqbalance openat: /proc/irq/65/smp_affinity
irqbalance openat: /proc/irq/61/smp_affinity
irqbalance openat: /proc/irq/51/smp_affinity
irqbalance openat: /proc/irq/50/smp_affinity
irqbalance openat: /proc/irq/76/smp_affinity
irqbalance openat: /proc/irq/54/smp_affinity
irqbalance openat: /proc/irq/68/smp_affinity
irqbalance openat: /proc/irq/10/smp_affinity
irqbalance openat: /proc/irq/12/smp_affinity
irqbalance openat: /proc/irq/49/smp_affinity
irqbalance openat: /proc/irq/60/smp_affinity
systemd-oomd openat: /proc/meminfo
systemd-oomd openat: /proc/meminfo
systemd-oomd openat: /sys/fs/cgroup/user.slice/user-
systemd-oomd openat: /sys/fs/cgroup/user.slice/user-
systemd-oomd openat: /sys/fs/cgroup/user.slice/user-
systemd-oomd openat: /sys/fs/cgroup/user.slice/user-
systemd-oomd openat: /sys/fs/cgroup/user.slice/user-
systemd-oomd openat: /sys/fs/cgroup/user.slice/user-
systemd-oomd openat: /proc/meminfo
```
