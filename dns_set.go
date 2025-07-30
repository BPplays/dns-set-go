package dns_set

import "context"


var Providers = map[string]DnsAPI{
	"porkbun": &Porkbun{},
}


type Record struct {
	Domain    DomainSub
	Disabled    bool
	Type    string
	Content string
	TTL     string
	Prio    string
	Notes   string
}

func (r *Record) SetDefaults() {
    if r.TTL == "" {
        r.TTL = "300"
    }
}

type Auth struct {
	Username string
	Password string

	ApiKey string
	ApiSecretKey string
}

type DomainSub struct {
	Domain string
	Sub string
}

func (d DomainSub) full() string {
	if d.Sub == "" { return d.Domain }
	if d.Domain == "" { return d.Sub }
	return d.Sub + "." + d.Domain
}

type DnsAPI interface {
	SetAuth(Auth) DnsAPI

	SetDns(context.Context, []Record) error
	GetDns(context.Context, []DomainSub) ([]Record, error)
	GetSuppoertedRecords() ([]string, error)
}

