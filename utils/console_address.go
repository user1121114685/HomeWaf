package utils

import (
	"net"
	"sort"
	"strconv"
	"strings"
)

// ConsoleURLs lists local interface addresses without contacting an IP discovery service.
func ConsoleURLs(port int, tls bool, entry string) []string {
	hosts := ConsoleIPs()
	scheme := "http"
	if tls {
		scheme = "https"
	}
	suffix := "/"
	if entry != "" {
		suffix += strings.Trim(entry, "/") + "/"
	}
	var urls []string
	for _, host := range hosts {
		urls = append(urls, scheme+"://"+net.JoinHostPort(host, strconv.Itoa(port))+suffix)
	}
	return urls
}

func ConsoleIPs() []string {
	interfaces, _ := net.Interfaces()
	var hosts []string
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, _ := iface.Addrs()
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err == nil && ip.IsGlobalUnicast() {
				hosts = append(hosts, ip.String())
			}
		}
	}
	sort.Slice(hosts, func(i, j int) bool {
		a, b := net.ParseIP(hosts[i]), net.ParseIP(hosts[j])
		if (a.To4() != nil) != (b.To4() != nil) {
			return a.To4() != nil
		}
		if a.IsPrivate() != b.IsPrivate() {
			return a.IsPrivate()
		}
		return hosts[i] < hosts[j]
	})
	var ips []string
	seen := map[string]bool{}
	for _, host := range hosts {
		if !seen[host] {
			ips = append(ips, host)
			seen[host] = true
		}
	}
	return ips
}
