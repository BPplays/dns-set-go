package dns_set

import "context"


var Providers = map[string]DnsAPI{
	"porkbun": &Porkbun{},
}


type Record struct {
	Domain    string
	Name    string
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

type DnsAPI interface {
	SetAuth(Auth) DnsAPI

	SetDns(context.Context, []Record) error
	GetDns(context.Context, []string) ([]Record, error)
	GetSuppoertedRecords() ([]string, error)
}

