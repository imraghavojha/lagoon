package cmd

import (
	"regexp"
	"sort"
	"strings"
)

var portPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:--port|-p)\s+([0-9]{2,5})\b`),
	regexp.MustCompile(`(?i)(?:--port|-p)=([0-9]{2,5})\b`),
	regexp.MustCompile(`(?i)\bPORT=([0-9]{2,5})\b`),
	regexp.MustCompile(`(?i)\b(?:localhost|127\.0\.0\.1|0\.0\.0\.0):([0-9]{2,5})\b`),
	regexp.MustCompile(`(?i)\b(?:--listen|--addr|--address|--bind)\s+(?:[^\s:]+:)?([0-9]{2,5})\b`),
	regexp.MustCompile(`(?i)\b(?:--listen|--addr|--address|--bind)=(?:[^\s:]+:)?([0-9]{2,5})\b`),
}

var httpServerPortPattern = regexp.MustCompile(`(?i)\bhttp\.server\s+([0-9]{2,5})\b`)

func inferPorts(command string) []string {
	seen := map[string]bool{}
	var ports []string
	add := func(port string) {
		if !validPort(port) || seen[port] {
			return
		}
		seen[port] = true
		ports = append(ports, port)
	}
	for _, pattern := range portPatterns {
		for _, match := range pattern.FindAllStringSubmatch(command, -1) {
			if len(match) > 1 {
				add(match[1])
			}
		}
	}
	for _, match := range httpServerPortPattern.FindAllStringSubmatch(command, -1) {
		if len(match) > 1 {
			add(match[1])
		}
	}
	sort.SliceStable(ports, func(i, j int) bool { return ports[i] < ports[j] })
	return ports
}

func portsLabel(ports []string) string {
	if len(ports) == 0 {
		return "unknown"
	}
	return strings.Join(ports, ",")
}

func validPort(port string) bool {
	if port == "" || len(port) > 5 {
		return false
	}
	value := 0
	for _, r := range port {
		if r < '0' || r > '9' {
			return false
		}
		value = value*10 + int(r-'0')
	}
	return value > 0 && value <= 65535
}
