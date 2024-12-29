#include <linux/if_ether.h>
#include <linux/if_packet.h>
#include <bpf/bpf_endian.h>
#include <linux/ip.h>
#include <linux/ipv6.h>
#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>

char LICENSE[] SEC("license") = "GPL";

SEC("xdp")
int openat(struct xdp_md *ctx) {
	void* start = (void*)(long)ctx->data;
	void* end = (void*)(long)ctx->data_end;

	// first convert to ethernet header.
	struct ethhdr *eth = start;
	int ethsize = sizeof(struct ethhdr);
	if (start + ethsize > end) {
		return 0;
	}

	if (eth->h_proto == bpf_htons(ETH_P_IP)) {
		struct iphdr *ip = (start + ethsize);
		int ipsize = sizeof(struct iphdr);
		if (start + ethsize + ipsize > end) {
			return 0;
		}

		// Print the source IP address in readable format
		bpf_printk("Received Source IP: %d.%d.%d.%d\n",
		       (ip->saddr & 0xFF),
		       (ip->saddr >> 8) & 0xFF,
		       (ip->saddr >> 16) & 0xFF,
		       (ip->saddr >> 24) & 0xFF
	        );
	} else {
		struct ipv6hdr *ipv6 = (start + ethsize);
		int ipsize = sizeof(struct ipv6hdr);
		if (start + ethsize + ipsize > end) {
			return 0;
		}
		// Print the source IP address in readable format
		bpf_printk("Recieved Source IPv6: %04x:%04x:%04x:%04x:%04x:%04x:%04x:%04x",
				(ipv6->saddr.in6_u.u6_addr16[0]),
				(ipv6->saddr.in6_u.u6_addr16[1]),
				(ipv6->saddr.in6_u.u6_addr16[2]),
				(ipv6->saddr.in6_u.u6_addr16[3]),
				(ipv6->saddr.in6_u.u6_addr16[4]),
				(ipv6->saddr.in6_u.u6_addr16[5]),
				(ipv6->saddr.in6_u.u6_addr16[6]),
				(ipv6->saddr.in6_u.u6_addr16[7])
		);
	}
	
	// continue with the linux network stack.
	// other options are:
	//
	//	XDP_ABORTED,
	//	XDP_DROP,
	//	XDP_PASS,
	//	XDP_TX,
	//	XDP_REDIRECT,
	return XDP_PASS; 
}

