package resolver

import (
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/sohidul/dns-server/internal/db/models"
)

// matchesCondition reports whether a steering rule's condition applies to the
// given query attributes.
func matchesCondition(rule models.SteeringRule, domain, clientIP, qtypeStr string) bool {
	switch rule.ConditionType {
	case "Domain":
		return matchDomain(rule.ConditionValue, domain)
	case "Client IP":
		return matchCIDR(rule.ConditionValue, clientIP)
	case "Query Type":
		for _, t := range strings.Split(rule.ConditionValue, ",") {
			if strings.EqualFold(strings.TrimSpace(t), qtypeStr) {
				return true
			}
		}
	case "Time Range":
		return matchTimeRange(rule.ConditionValue)
	}
	return false
}

// matchDomain supports exact matches and "*." wildcard suffixes.
func matchDomain(pattern, domain string) bool {
	pattern = strings.ToLower(strings.TrimSpace(pattern))
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[2:]
		return domain == suffix || strings.HasSuffix(domain, "."+suffix)
	}
	return domain == pattern
}

// matchCIDR matches an IP against a CIDR range or a bare IP literal.
func matchCIDR(cidr, ip string) bool {
	target := net.ParseIP(ip)
	if target == nil {
		return false
	}
	if _, network, err := net.ParseCIDR(cidr); err == nil {
		return network.Contains(target)
	}
	parsed := net.ParseIP(cidr)
	return parsed != nil && parsed.Equal(target)
}

// matchTimeRange matches the current local time against an "HH:MM-HH:MM" range.
func matchTimeRange(r string) bool {
	parts := strings.SplitN(r, "-", 2)
	if len(parts) != 2 {
		return false
	}
	now := time.Now()
	cur := now.Hour()*60 + now.Minute()
	start := parseHHMM(parts[0])
	end := parseHHMM(parts[1])
	return start >= 0 && end >= 0 && cur >= start && cur <= end
}

func parseHHMM(s string) int {
	p := strings.SplitN(strings.TrimSpace(s), ":", 2)
	if len(p) != 2 {
		return -1
	}
	h, err1 := strconv.Atoi(p[0])
	m, err2 := strconv.Atoi(p[1])
	if err1 != nil || err2 != nil || h < 0 || h > 23 || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}
