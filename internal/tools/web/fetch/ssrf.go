package fetch

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

var privateCIDRs []*net.IPNet

func init() {
	cidrStrings := []string{
		// IPv4 Private & Reserved
		"0.0.0.0/8",
		"10.0.0.0/8",
		"100.64.0.0/10",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"172.16.0.0/12",
		"192.0.0.0/24",
		"192.0.2.0/24",
		"192.168.0.0/16",
		"198.18.0.0/15",
		"198.51.100.0/24",
		"203.0.113.0/24",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"255.255.255.255/32",
		// IPv6 Private & Reserved
		"::1/128",
		"::/128",
		"fc00::/7",
		"fe80::/10",
		"ff00::/8",
		"2001:db8::/32",
	}

	for _, cidr := range cidrStrings {
		_, ipNet, err := net.ParseCIDR(cidr)
		if err == nil {
			privateCIDRs = append(privateCIDRs, ipNet)
		}
	}
}

// IsPrivateIP checks whether an IP address belongs to a private, loopback, multicast, or reserved range.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return true
	}

	// Unwrap IPv4-mapped IPv6 addresses (e.g. ::ffff:127.0.0.1)
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() ||
		ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}

	for _, cidr := range privateCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}

	return false
}

// ValidateURL checks whether a URL is syntactically valid, has an allowed scheme (http/https),
// and does not point to a private IP (unless allowLocal is true).
func ValidateURL(rawURL string, allowLocal bool) error {
	if strings.TrimSpace(rawURL) == "" {
		return errors.New("url cannot be empty")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("unsupported url scheme %q (only http and https are allowed)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return errors.New("url host cannot be empty")
	}

	if !allowLocal {
		if strings.EqualFold(host, "localhost") {
			return errors.New("access to localhost is blocked (SSRF guard)")
		}

		ip := net.ParseIP(host)
		if ip != nil {
			if IsPrivateIP(ip) {
				return fmt.Errorf("access to private IP %s is blocked (SSRF guard)", ip.String())
			}
		}
	}

	return nil
}

// NewSafeTransport returns an http.Transport configured with SSRF protection on connection dial.
func NewSafeTransport(allowLocal bool, timeout time.Duration) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			if allowLocal {
				return nil
			}
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				host = address
			}
			ip := net.ParseIP(host)
			if ip != nil && IsPrivateIP(ip) {
				return fmt.Errorf("ssrf: connection to private IP %s is blocked", ip.String())
			}
			return nil
		},
	}

	transport := &http.Transport{
		Proxy:                 nil,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if allowLocal {
				return dialer.DialContext(ctx, network, addr)
			}

			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("invalid address %q: %w", addr, err)
			}

			// Resolve IP addresses and verify none are private before dialing
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("dns resolution failed for %q: %w", host, err)
			}

			if len(ips) == 0 {
				return nil, fmt.Errorf("no IP addresses resolved for %q", host)
			}

			for _, ipAddr := range ips {
				if IsPrivateIP(ipAddr.IP) {
					return nil, fmt.Errorf("ssrf: host %q resolved to private IP %s", host, ipAddr.IP.String())
				}
			}

			// Dial the first safe resolved IP explicitly to prevent DNS rebinding
			targetAddr := net.JoinHostPort(ips[0].IP.String(), port)
			return dialer.DialContext(ctx, network, targetAddr)
		},
	}

	return transport
}
