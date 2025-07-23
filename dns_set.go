package dns_set

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
	New(Auth) DnsAPI

	setDns(Record) error
	getDns(string) ([]Record, error)
}


func main() {


}
