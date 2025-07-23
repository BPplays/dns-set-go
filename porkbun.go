package dns_set

import (
	"github.com/nrdcg/porkbun"
)

type Porkbun struct {
	self *porkbun.Client
}

func (p Porkbun) setAuth(a Auth) DnsAPI {

	p.self = porkbun.New(a.api_secret_key, a.api_key)
	return p
}

func (p Porkbun) setDns(record Record) error {
	return nil
}

func (p Porkbun) getDns(s string) ([]Record, error) {
	return []Record{}, nil
}
