#include "openatsnoop.skel.h"
#include "events.h"
#include <stdio.h>

int debug_print(enum libbpf_print_level level, const char *format, va_list args) {
	return vfprintf(stdout, format, args);
}

int handle_event(void *ctx, void *data, size_t size) {
	struct event *event = data;
	fprintf(stdout, "%s openat: %s\n", event->parent_comm, event->requested_comm);
	return 0;
}

int main(int argc, char **arv) {
	int err = 0;
	struct openatsnoop_bpf *skeleton = NULL;
	struct ring_buffer *buffer = NULL;

	libbpf_set_print(debug_print);

	skeleton = openatsnoop_bpf__open_and_load();
	if (!skeleton) {
		goto done;
	}

	err = openatsnoop_bpf__attach(skeleton);
	if (err) {
		goto done;
	}

	buffer = ring_buffer__new(bpf_map__fd(skeleton->maps.buffer), handle_event, 0, 0);
	if (!buffer) {
		goto done;
	}

	for (;;) {
		err = ring_buffer__poll(buffer, 10);
		if (err < 0) {
			break;
		}
	}

done:
	if (skeleton) openatsnoop_bpf__destroy(skeleton);
	if (buffer) ring_buffer__free(buffer);
	return -err;
}

