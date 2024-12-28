#include "vmlinux.h"
#include "events.h"

#include <bpf/bpf_tracing.h>
#include <bpf/bpf_core_read.h>

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1024);
} buffer SEC(".maps");

char LICENSE[] SEC("license") = "GPL";

SEC("tracepoint/syscalls/sys_enter_openat")
int openat(struct trace_event_raw_sys_enter *ctx) {
	struct event *event = bpf_ringbuf_reserve(&buffer, sizeof(struct event), 0);
	if (!event) {
		// failed to allocate memory for next event.
		return 0;
	}
	bpf_get_current_comm(&event->parent_comm, sizeof(event->parent_comm));
	bpf_probe_read_str(&event->requested_comm, sizeof(event->requested_comm), (void*)ctx->args[1]);
	bpf_ringbuf_submit(event, 0);
	return 0;
}

