package dns_set

import "context"

var Providers = map[string]DnsAPI{
	"porkbun": Porkbun{},
}

type Record struct {
	Name    string
	Disabled    bool
	Type    string
	Content string
	TTL     string
	Prio    string
	Notes   string
}

type Auth struct {
	username string
	password string

	api_key string
	api_secret_key string
}

type DnsAPI interface {
	setAuth(Auth) DnsAPI

	setDns(context.Context, []Record) error
	getDns(context.Context, string) ([]Record, error)
}


func main() {



}
