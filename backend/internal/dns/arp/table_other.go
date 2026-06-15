//go:build !linux

package arp

// readARPTable returns an empty table on platforms without /proc/net/arp.
// MAC resolution is a Linux-only feature; elsewhere Lookup yields "".
func readARPTable() (map[string]string, error) {
	return map[string]string{}, nil
}
