//go:build linux

package arp

import (
	"bufio"
	"os"
	"strings"
)

const emptyMAC = "00:00:00:00:00:00"

// readARPTable parses /proc/net/arp into an IP -> MAC map. Incomplete entries
// (all-zero MAC) are skipped.
func readARPTable() (map[string]string, error) {
	f, err := os.Open("/proc/net/arp")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	table := make(map[string]string)
	scanner := bufio.NewScanner(f)
	scanner.Scan() // skip header line
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 {
			continue
		}
		ip, mac := fields[0], fields[3]
		if mac != emptyMAC {
			table[ip] = mac
		}
	}
	return table, scanner.Err()
}
