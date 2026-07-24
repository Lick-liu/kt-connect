package util

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"net"
	"regexp"
	"strconv"
	"strings"
)

const IpAddrPattern = "[0-9]+\\.[0-9]+\\.[0-9]+\\.[0-9]+"

// GetRandomTcpPort get pod random ssh port
// Probe with net.Listen rather than net.Dial: a port inside an OS-excluded
// range (e.g. Windows Hyper-V reserved port ranges) accepts no connection,
// so dialing reports it as "free" while listening on it still fails.
func GetRandomTcpPort() int {
	port := 0
	for i := 0; i < 20; i++ {
		port = RandomPort()
		listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			log.Debug().Msgf("Port %d not available: %s", port, err)
			continue
		}
		_ = listener.Close()
		log.Debug().Msgf("Using port %d", port)
		return port
	}
	log.Warn().Msgf("No verified free port found, using random port %d", port)
	return port
}

// ParsePortMapping parse <port> or <localPort>:<removePort> parameter
func ParsePortMapping(exposePort string) (int, int, error) {
	localPort := exposePort
	remotePort := exposePort
	ports := strings.SplitN(exposePort, ":", 2)
	if len(ports) > 1 {
		localPort = ports[0]
		remotePort = ports[1]
	}
	lp, err := strconv.Atoi(localPort)
	if err != nil {
		return -1, -1, fmt.Errorf("local port '%s' is not a number", localPort)
	}
	rp, err := strconv.Atoi(remotePort)
	if err != nil {
		return -1, -1, fmt.Errorf("remote port '%s' is not a number", remotePort)
	}
	return lp, rp, nil
}

// FindBrokenLocalPort Check if all ports has process listening to
// Return empty string if all ports are listened, otherwise return the first broken port
func FindBrokenLocalPort(exposePorts string) string {
	portPairs := strings.Split(exposePorts, ",")
	for _, exposePort := range portPairs {
		localPort := strings.Split(exposePort, ":")[0]
		conn, err := net.Dial("tcp", fmt.Sprintf(":%s", localPort))
		if err == nil {
			_ = conn.Close()
		} else {
			return localPort
		}
	}
	return ""
}

// FindInvalidRemotePort Check if all ports exist in provide service
func FindInvalidRemotePort(exposePorts string, svcPorts map[int]string) string {
	validPorts := make([]string, 0)
	for p := range svcPorts {
		validPorts = append(validPorts, strconv.Itoa(p))
	}
	log.Debug().Msgf("Service target ports: %v", validPorts)

	portPairs := strings.Split(exposePorts, ",")
	for _, exposePort := range portPairs {
		splitPorts := strings.Split(exposePort, ":")
		remotePort := splitPorts[0]
		if len(splitPorts) > 1 {
			remotePort = splitPorts[1]
		}
		if !Contains(validPorts, remotePort) {
			return remotePort
		}
	}
	return ""
}

// IsValidIp check if specified ip address valid
func IsValidIp(ip string) bool {
	if ok, err := regexp.MatchString("^" + IpAddrPattern + "$", ip); ok && err == nil {
		return true
	}
	return false
}

// ExtractHostIp Get host ip address from url
func ExtractHostIp(url string) string {
	if !strings.Contains(url, ":") {
		return ""
	}
	host := strings.Trim(strings.Split(url, ":")[1], "/")
	if IsValidIp(host) {
		return host
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return ""
	}
	for _, ip := range ips {
		// skip ipv6
		if IsValidIp(ip.String()) {
			return ip.String()
		}
	}
	return ""
}
