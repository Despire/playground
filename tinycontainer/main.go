package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"runtime"
	"syscall"

	"github.com/coreos/go-iptables/iptables"
	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

const (
	// ip address of the host to resolve the dns queries.
	etcResolve = `
nameserver 192.168.72.1
search .
`
)

const (
	lower  = "./overlay/image"
	upper  = "./overlay/container/upper"
	work   = "./overlay/container/work"
	merged = "./overlay/container/merged"
)

const (
	myDockerInterface = "my-docker0"
	gatewayAddr       = "10.0.0.1/8"
	veth0Addr         = "10.0.0.2/8"
)

var (
	myDockerInterfacePostroutingRules = [...][]string{
		// allow communicating with outside world with NAT.
		// iptables -t nat -A POSTROUTING -s 'my-docker0-ip' ! -o my-docker0 -j MASQUERADE
		[]string{"-s", gatewayAddr, "!", "-o", myDockerInterface, "-j", "MASQUERADE"},
	}

	myDockerInterfaceForwardRules = [...][]string{
		// allow packets that are part of established connections to be forwarded.
		// iptables -A FORWARD -o my-docker0 -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT
		[]string{"-o", myDockerInterface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"},
		// allow forwarding packets originating from my-docker0 to be forwarded to other interfaces.
		// iptables -A FORWARD -i my-docker0 ! -o my-docker0 -j ACCEPT
		[]string{"-i", myDockerInterface, "!", "-o", myDockerInterface, "-j", "ACCEPT"},
		// allow communication within containers.
		// iptables -A FORWARD -i my-docker0 -o my-docker0 -j ACCEPT
		[]string{"-i", myDockerInterface, "-o", myDockerInterface, "-j", "ACCEPT"},
	}
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("failed to run container: %v", err)
	}
}

func run(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("no arguments recieved, expected <run> <args...>")
	}

	fmt.Printf("pid: %v\n", os.Getpid())

	switch args[0] {
	case "run":
		return prepare(args[1:])
	case "interactive":
		return interact(args[1:])
	default:
		return fmt.Errorf("unrecognized command: %v, expected usage <run> <args...>", args[0])
	}
}

func prepare(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("no container arguments, expected <args...>")
	}

	// so that we don't switch namespaces while initializing.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	this, newns, err := setupNetworkNamespaces()
	if err != nil {
		return fmt.Errorf("failed to setup network namespaces: %w", err)
	}
	defer newns.Close()
	defer this.Close()
	defer netns.Set(this)

	cleanup, err := setupContainerNetwork(this, newns)
	defer cleanup()
	if err != nil {
		return fmt.Errorf("failed to setup container networking: %w", err)
	}

	umount, err := createOverlay(lower, upper, work, merged)
	defer umount()
	if err != nil {
		return fmt.Errorf("failed to create overlay fs: %w", err)
	}

	cmd := exec.Command("/proc/self/exe", append([]string{"interactive"}, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		fmt.Sprintf("TERM=%s", os.Getenv("TERM")),
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:   syscall.CLONE_NEWUTS | syscall.CLONE_NEWNS | syscall.CLONE_NEWPID,
		Unshareflags: syscall.CLONE_NEWNS,
	}
	return cmd.Run()
}

func interact(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("no container arguments, expected <args...>")
	}

	if err := syscall.Sethostname([]byte("container")); err != nil {
		return fmt.Errorf("failed to change container hostname: %w", err)
	}

	if err := syscall.Chroot(merged); err != nil {
		return fmt.Errorf("failed to chroot inside container: %w", err)
	}

	if err := syscall.Chdir("/"); err != nil {
		return fmt.Errorf("failed to change dir to '/' inside container: %w", err)
	}

	if err := syscall.Mount("proc", "/proc", "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc dir inside container: %w", err)
	}

	defer func() {
		if err := syscall.Unmount("/proc", 0); err != nil {
			fmt.Printf("failed to unmount proc inside container: %w", err)
		}
	}()

	// copy over resolv.conf
	if err := os.WriteFile("/etc/resolv.conf", []byte(etcResolve), 0644); err != nil {
		return fmt.Errorf("failed to inject resolve.conf: %w", err)
	}

	fmt.Printf("executing command: %v with args: %v\n", args[0], args[1:])

	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func createOverlay(lower, upper, work, merged string) (func() error, error) {
	// mount -t overlay overlay -o lowerdir=./image/,upperdir=./container/upper/,workdir=./container/work/ ./container/merged/
	err := syscall.Mount(
		lower,
		merged,
		"overlay",
		0,
		fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", lower, upper, work),
	)
	if err != nil {
		return func() error { return nil }, err
	}

	return func() error { return syscall.Unmount(merged, 0) }, nil
}

func setupContainerNetwork(parent, child netns.NsHandle) (func() error, error) {
	var (
		cleanupVeth   = func() error { return nil }
		cleanupBridge = func() error { return nil }
		cleanupFunc   = func() error {
			var all error
			all = errors.Join(all, cleanupVeth())
			all = errors.Join(all, cleanupRules())
			all = errors.Join(all, cleanupBridge())
			return all
		}
	)

	// create bridge
	la := netlink.NewLinkAttrs()
	la.Name = myDockerInterface
	myDockerBridge := &netlink.Bridge{
		LinkAttrs: la,
	}

	if err := netlink.LinkAdd(myDockerBridge); err != nil {
		return cleanupFunc, fmt.Errorf("failed to setup my-docker0 bridge: %w", err)
	}

	cleanupBridge = func() error { return netlink.LinkDel(myDockerBridge) }

	bridgeAddr, err := netlink.ParseAddr(gatewayAddr)
	if err != nil {
		return cleanupFunc, fmt.Errorf("failed to parse my-docker0 bridge cidr: %w", err)
	}

	if err := netlink.AddrAdd(myDockerBridge, bridgeAddr); err != nil {
		return cleanupFunc, fmt.Errorf("failed to set addr for my-docker0 bridge: %w", err)
	}

	if err := netlink.LinkSetUp(myDockerBridge); err != nil {
		return cleanupFunc, fmt.Errorf("failed to bring up my-docker0 bridge: %w", err)
	}

	ipt, err := iptables.New()
	if err != nil {
		return cleanupFunc, fmt.Errorf("failed to create iptables client: %w", err)
	}

	for _, r := range myDockerInterfacePostroutingRules {
		if err := ipt.Append("nat", "POSTROUTING", r...); err != nil {
			return cleanupFunc, fmt.Errorf("failed to create NAT POSTROUTING rule %v: %w", r, err)
		}
	}

	for _, r := range myDockerInterfaceForwardRules {
		if err := ipt.Append("filter", "FORWARD", r...); err != nil {
			return cleanupFunc, fmt.Errorf("failed to create ip FORWARD rule %v: %w", r, err)
		}
	}

	// create veth pair
	vethLa := netlink.NewLinkAttrs()
	vethLa.Name = "veth0"
	veth0 := &netlink.Veth{
		LinkAttrs: vethLa,
		PeerName:  "veth1",
	}

	if err := netlink.LinkAdd(veth0); err != nil {
		return cleanupFunc, fmt.Errorf("failed to setup container veth pair: %w", err)
	}

	cleanupVeth = func() error { return netlink.LinkDel(veth0) }

	veth1, err := netlink.LinkByName("veth1")
	if err != nil {
		return cleanupFunc, fmt.Errorf("failed to find veth01: %w", err)
	}

	if err := netlink.LinkSetUp(veth1); err != nil {
		return cleanupFunc, fmt.Errorf("failed to bring up veth1: %w", err)
	}

	if err := netlink.LinkSetMaster(veth1, myDockerBridge); err != nil {
		return cleanupFunc, fmt.Errorf("failed to set my-docer0 bridge as master for veth1: %w", err)
	}

	// move veth0 into the container network namespace configure
	// it, bring up the loopback interface within that namespace
	// and set default route via created bridge.

	if err := netlink.LinkSetNsFd(veth0, int(child)); err != nil {
		return cleanupFunc, fmt.Errorf("failed to move veth0 to container network namespace: %w", err)
	}

	if err := netns.Set(child); err != nil {
		return cleanupFunc, fmt.Errorf("failed to switch to container network namespace: %w", err)
	}

	cleanupVeth = func() error {
		var all error
		all = errors.Join(all, netlink.LinkDel(veth0))
		all = errors.Join(all, netns.Set(parent))
		return all
	}

	veth0Addr, err := netlink.ParseAddr(veth0Addr)
	if err != nil {
		return cleanupFunc, fmt.Errorf("failed to parse veth0 addr: %w", err)
	}

	if err := netlink.AddrAdd(veth0, veth0Addr); err != nil {
		return cleanupFunc, fmt.Errorf("failed to assign address %s to veth0: %w", veth0Addr, err)
	}

	if err := netlink.LinkSetUp(veth0); err != nil {
		return cleanupFunc, fmt.Errorf("failed to bring up veth0: %w", err)
	}

	lo, err := netlink.LinkByName("lo")
	if err != nil {
		return cleanupFunc, fmt.Errorf("failed to find loopback interface inside container network namespace: %w", err)
	}

	if err := netlink.LinkSetUp(lo); err != nil {
		return cleanupFunc, fmt.Errorf("failed to bring up loopback interface inside container network namespace: %w", err)
	}

	err = netlink.RouteAdd(&netlink.Route{
		Scope: netlink.SCOPE_UNIVERSE,
		Gw:    net.ParseIP(gatewayAddr[:len(gatewayAddr-2)]),
	})

	if err != nil {
		return cleanupFunc, fmt.Errorf("failed to add my-docker0 bridge as default route inside container network namespace: %w", err)
	}

	return cleanupFunc, nil
}

func setupNetworkNamespaces() (netns.NsHandle, netns.NsHandle, error) {
	current, err := netns.Get()
	if err != nil {
		return -1, -1, fmt.Errorf("failed to get current namespace: %w", err)
	}

	newns, err := netns.New()
	if err != nil {
		return -1, -1, fmt.Errorf("failed to create new namespace: %w", err)
	}

	if err := netns.Set(current); err != nil {
		return -1, -1, fmt.Errorf("failed to switch back to current namespace: %w", err)
	}

	return current, newns, nil
}

func cleanupRules() error {
	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to create iptables client: %w", err)
	}

	var errAll error

	for _, r := range myDockerInterfacePostroutingRules {
		if err := ipt.Delete("nat", "POSTROUTING", r...); err != nil {
			errAll = errors.Join(errAll, fmt.Errorf("failed to create NAT POSTROUTING rule %v: %w", r, err))
		}
	}

	for _, r := range myDockerInterfaceForwardRules {
		if err := ipt.Delete("filter", "FORWARD", r...); err != nil {
			errAll = errors.Join(errAll, fmt.Errorf("failed to create ip FORWARD rule %v: %w", r, err))
		}
	}

	return errAll
}
