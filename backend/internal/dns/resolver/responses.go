package resolver

import (
	"net"
	"strconv"

	"github.com/miekg/dns"
)

// answerBlocked returns the configured response for a blocked domain: either
// NXDOMAIN or an A record pointing at 0.0.0.0.
func (r *Resolver) answerBlocked(req *dns.Msg) *dns.Msg {
	if r.blockNX {
		return r.answerNXDOMAIN(req)
	}
	if req.Question[0].Qtype == dns.TypeA {
		return r.answerAddress(req, blockedIPv4, 60)
	}
	// For non-A queries (e.g. AAAA) there is no 0.0.0.0 equivalent; NODATA
	// keeps the domain effectively unreachable without lying about the type.
	return r.answerNoData(req)
}

// answerAddress builds a NOERROR response with a single A or AAAA record chosen
// by the IP family. An empty or mismatched IP yields NODATA.
func (r *Resolver) answerAddress(req *dns.Msg, ip string, ttl uint32) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(req)

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return r.answerNoData(req)
	}
	name := req.Question[0].Name
	qtype := req.Question[0].Qtype
	ttlStr := strconv.FormatUint(uint64(ttl), 10)

	var rrText string
	switch {
	case parsed.To4() != nil && qtype == dns.TypeA:
		rrText = name + " " + ttlStr + " IN A " + ip
	case parsed.To4() == nil && qtype == dns.TypeAAAA:
		rrText = name + " " + ttlStr + " IN AAAA " + ip
	default:
		return r.answerNoData(req)
	}

	if rr, err := dns.NewRR(rrText); err == nil {
		resp.Answer = append(resp.Answer, rr)
	}
	return resp
}

// answerNXDOMAIN builds a negative (name does not exist) response.
func (r *Resolver) answerNXDOMAIN(req *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetRcode(req, dns.RcodeNameError)
	return resp
}

// answerNoData builds an empty NOERROR response (name exists, no record of the
// requested type).
func (r *Resolver) answerNoData(req *dns.Msg) *dns.Msg {
	resp := new(dns.Msg)
	resp.SetReply(req)
	return resp
}
