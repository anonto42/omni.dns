package records

// Record is the aggregate root for a custom DNS record. It is composed of
// validated value objects, so a constructed Record is always valid.
type Record struct {
	domain Domain
	ip     IP
}

// New builds a Record from raw domain and IP strings, enforcing all invariants.
func New(domain, ip string) (Record, error) {
	d, err := NewDomain(domain)
	if err != nil {
		return Record{}, err
	}
	addr, err := NewIP(ip)
	if err != nil {
		return Record{}, err
	}
	return Record{domain: d, ip: addr}, nil
}

// NewFromValues builds a Record from already-validated value objects, e.g. when
// reconstituting from storage.
func NewFromValues(domain Domain, ip IP) Record {
	return Record{domain: domain, ip: ip}
}

// Domain returns the record's domain value object.
func (r Record) Domain() Domain { return r.domain }

// IP returns the record's IP value object.
func (r Record) IP() IP { return r.ip }
