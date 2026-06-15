package resolver

import (
	"testing"

	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sohidul/dns-server/internal/modules/resolver/engine/cache"
	"github.com/sohidul/dns-server/internal/modules/resolver/engine/forwarder"
	"github.com/sohidul/dns-server/internal/shared/models"
)

// --- fakes ---------------------------------------------------------------

type fakeBlocklist struct{ blocked map[string]bool }

func (f fakeBlocklist) IsBlocked(d string) bool { return f.blocked[d] }

type fakeRecords struct {
	v4 map[string]string // domain -> IPv4
	v6 map[string]string // domain -> IPv6
}

func (f fakeRecords) Lookup(domain string, qtype uint16) (string, bool, bool) {
	switch qtype {
	case dns.TypeA:
		if ip, ok := f.v4[domain]; ok {
			return ip, true, false
		}
		_, other := f.v6[domain]
		return "", false, other
	case dns.TypeAAAA:
		if ip, ok := f.v6[domain]; ok {
			return ip, true, false
		}
		_, other := f.v4[domain]
		return "", false, other
	}
	return "", false, false
}

type fakeSteering struct{ rules []models.SteeringRule }

func (f fakeSteering) Rules() []models.SteeringRule { return f.rules }

type fakeLogger struct{ entries []models.QueryLog }

func (f *fakeLogger) LogQuery(q models.QueryLog) { f.entries = append(f.entries, q) }

type fakeMAC struct{}

func (fakeMAC) Lookup(string) string { return "" }

func newResolver(t *testing.T, d Deps) (*Resolver, *fakeLogger) {
	t.Helper()
	lg := &fakeLogger{}
	d.Logger = lg
	d.MAC = fakeMAC{}
	if d.Cache == nil {
		d.Cache = cache.New(100)
		t.Cleanup(d.Cache.Close)
	}
	if d.Pool == nil {
		d.Pool = forwarder.NewPool([]forwarder.Upstream{{Addr: "192.0.2.1:53"}})
		t.Cleanup(d.Pool.Close)
	}
	if d.Blocklist == nil {
		d.Blocklist = fakeBlocklist{}
	}
	if d.Records == nil {
		d.Records = fakeRecords{}
	}
	if d.Steering == nil {
		d.Steering = fakeSteering{}
	}
	return New(d), lg
}

func query(name string, qtype uint16) *dns.Msg {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), qtype)
	return m
}

// --- tests ---------------------------------------------------------------

func TestCustomRecordA(t *testing.T) {
	r, _ := newResolver(t, Deps{Records: fakeRecords{v4: map[string]string{"box.local": "192.168.1.10"}}})
	resp := r.Resolve(query("box.local", dns.TypeA), "192.168.1.2", "UDP")
	require.NotNil(t, resp)
	require.Len(t, resp.Answer, 1)
	a, ok := resp.Answer[0].(*dns.A)
	require.True(t, ok)
	assert.Equal(t, "192.168.1.10", a.A.String())
}

func TestCustomRecordAAAA(t *testing.T) {
	r, _ := newResolver(t, Deps{Records: fakeRecords{v6: map[string]string{"box.local": "fd00::1"}}})
	resp := r.Resolve(query("box.local", dns.TypeAAAA), "192.168.1.2", "UDP")
	require.NotNil(t, resp)
	require.Len(t, resp.Answer, 1)
	aaaa, ok := resp.Answer[0].(*dns.AAAA)
	require.True(t, ok)
	assert.Equal(t, "fd00::1", aaaa.AAAA.String())
}

// An AAAA query against an A-only custom record must return NODATA (empty
// NOERROR), not fall through to upstream and not return an A record.
func TestCustomRecordAAAAOverA_ReturnsNoData(t *testing.T) {
	r, lg := newResolver(t, Deps{Records: fakeRecords{v4: map[string]string{"box.local": "192.168.1.10"}}})
	resp := r.Resolve(query("box.local", dns.TypeAAAA), "192.168.1.2", "UDP")
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeSuccess, resp.Rcode)
	assert.Empty(t, resp.Answer, "AAAA over A-only record must be NODATA")
	require.Len(t, lg.entries, 1)
	assert.Equal(t, models.ActionCustom, lg.entries[0].Action)
}

func TestBlockedDomainDefault0000(t *testing.T) {
	r, _ := newResolver(t, Deps{Blocklist: fakeBlocklist{blocked: map[string]bool{"ads.example.com": true}}})
	resp := r.Resolve(query("ads.example.com", dns.TypeA), "192.168.1.2", "UDP")
	require.NotNil(t, resp)
	require.Len(t, resp.Answer, 1)
	a := resp.Answer[0].(*dns.A)
	assert.Equal(t, "0.0.0.0", a.A.String())
}

// A blocked domain queried for AAAA has no 0.0.0.0 analogue, so NODATA.
func TestBlockedDomainAAAA_ReturnsNoData(t *testing.T) {
	r, _ := newResolver(t, Deps{Blocklist: fakeBlocklist{blocked: map[string]bool{"ads.example.com": true}}})
	resp := r.Resolve(query("ads.example.com", dns.TypeAAAA), "192.168.1.2", "UDP")
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeSuccess, resp.Rcode)
	assert.Empty(t, resp.Answer)
}

func TestBlockedDomainNXDOMAIN(t *testing.T) {
	r, _ := newResolver(t, Deps{
		BlockNX:   true,
		Blocklist: fakeBlocklist{blocked: map[string]bool{"ads.example.com": true}},
	})
	resp := r.Resolve(query("ads.example.com", dns.TypeA), "192.168.1.2", "UDP")
	require.NotNil(t, resp)
	assert.Equal(t, dns.RcodeNameError, resp.Rcode)
}

// A steering Redirect to an IPv4 target must answer AAAA queries with NODATA.
func TestSteeringRedirectAAAAFamilyMismatch(t *testing.T) {
	rules := []models.SteeringRule{{
		Name: "r", Enabled: true,
		ConditionType: "Domain", ConditionValue: "intranet.local",
		ActionType: "Redirect", ActionTarget: "10.0.0.5", Priority: 1,
	}}
	r, _ := newResolver(t, Deps{Steering: fakeSteering{rules: rules}})

	v4 := r.Resolve(query("intranet.local", dns.TypeA), "192.168.1.2", "UDP")
	require.Len(t, v4.Answer, 1)

	v6 := r.Resolve(query("intranet.local", dns.TypeAAAA), "192.168.1.2", "UDP")
	require.NotNil(t, v6)
	assert.Equal(t, dns.RcodeSuccess, v6.Rcode)
	assert.Empty(t, v6.Answer)
}
