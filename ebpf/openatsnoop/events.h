#ifndef EVENTS_H
#define EVENTS_H

#define MAX_COMM_LENGTH 32

struct event {
	char parent_comm[MAX_COMM_LENGTH];
	char requested_comm[MAX_COMM_LENGTH];
};

#endif
