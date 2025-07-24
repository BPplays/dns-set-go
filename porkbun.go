package dns_set

import (
	"context"

	"github.com/nrdcg/porkbun"
)

type Porkbun struct {
	self *porkbun.Client
}

func (p Porkbun) setAuth(a Auth) DnsAPI {

	p.self = porkbun.New(a.api_secret_key, a.api_key)
	return p
}

func (p Porkbun) setDns(ctx context.Context, records []Record) error {
	return nil
}

func (p Porkbun) getDns(ctx context.Context, s string) ([]Record, error) {
	return []Record{}, nil
}
