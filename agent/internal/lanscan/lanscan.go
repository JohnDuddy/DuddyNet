// Package lanscan performs a deliberately conservative discovery sweep of the
// configured subnet. It probes a small set of common, allowlisted ports on each
// host with bounded concurrency and short timeouts. It does NOT do aggressive
// full-range port scanning.
//
// Results are returned to the caller (CLI), which presents them for the user to
// confirm before anything is written to the store.
package lanscan

import (
	"context"
	"net"
	"sort"
	"strconv"
	"sync"
	"time"
)

// CommonPorts are the only ports probed during a scan.
var CommonPorts = []int{22, 80, 443, 445, 3389, 5000, 5001, 8123}

// Host is a discovered live host.
type Host struct {
	IP        string
	OpenPorts []int
	Hostname  string // reverse-DNS if resolvable
}

// Options tune a scan.
type Options struct {
	Ports       []int
	Timeout     time.Duration
	Concurrency int
	ResolveDNS  bool
}

func (o *Options) withDefaults() {
	if len(o.Ports) == 0 {
		o.Ports = CommonPorts
	}
	if o.Timeout <= 0 {
		o.Timeout = 600 * time.Millisecond
	}
	if o.Concurrency <= 0 {
		o.Concurrency = 32
	}
}

// Scan sweeps the given CIDR and returns hosts that have at least one open port.
// It respects ctx cancellation.
func Scan(ctx context.Context, cidr string, opts Options) ([]Host, error) {
	opts.withDefaults()

	ips, err := enumerate(cidr)
	if err != nil {
		return nil, err
	}

	sem := make(chan struct{}, opts.Concurrency)
	var (
		mu    sync.Mutex
		hosts []Host
		wg    sync.WaitGroup
	)

	for _, ip := range ips {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(ip string) {
			defer wg.Done()
			defer func() { <-sem }()
			open := probeHost(ctx, ip, opts)
			if len(open) == 0 {
				return
			}
			h := Host{IP: ip, OpenPorts: open}
			if opts.ResolveDNS {
				if names, err := net.LookupAddr(ip); err == nil && len(names) > 0 {
					h.Hostname = names[0]
				}
			}
			mu.Lock()
			hosts = append(hosts, h)
			mu.Unlock()
		}(ip)
	}
	wg.Wait()

	sort.Slice(hosts, func(i, j int) bool {
		return ipLess(hosts[i].IP, hosts[j].IP)
	})
	return hosts, nil
}

func probeHost(ctx context.Context, ip string, opts Options) []int {
	var open []int
	d := net.Dialer{Timeout: opts.Timeout}
	for _, p := range opts.Ports {
		select {
		case <-ctx.Done():
			return open
		default:
		}
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(p)))
		if err != nil {
			continue
		}
		_ = conn.Close()
		open = append(open, p)
	}
	return open
}

// enumerate returns all usable host IPs in a CIDR (excluding network and
// broadcast for IPv4 /31+ blocks). Guards against absurdly large ranges.
func enumerate(cidr string) ([]string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	var ips []string
	for ip := cloneIP(ipnet.IP.Mask(ipnet.Mask)); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
		if len(ips) > 4096 { // safety cap (~/20)
			break
		}
	}
	// Drop network + broadcast for typical IPv4 subnets.
	if len(ips) > 2 {
		ips = ips[1 : len(ips)-1]
	}
	return ips, nil
}

func cloneIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

func ipLess(a, b string) bool {
	ia, ib := net.ParseIP(a), net.ParseIP(b)
	if ia == nil || ib == nil {
		return a < b
	}
	ia, ib = ia.To16(), ib.To16()
	for i := range ia {
		if ia[i] != ib[i] {
			return ia[i] < ib[i]
		}
	}
	return false
}
