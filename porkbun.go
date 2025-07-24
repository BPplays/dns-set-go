package dns_set

import (
	"context"

	"github.com/nrdcg/porkbun"
)

type Porkbun struct {
	self *porkbun.Client
}

func (p Porkbun) SetAuth(a Auth) DnsAPI {

	p.self = porkbun.New(a.api_secret_key, a.api_key)
	return p
}

func (p Porkbun) setDns(records []Record) error {

	return nil
}

func (p Porkbun) SetDns(ctx context.Context, records []Record) error {
	return nil
}

func (p Porkbun) GetDns(ctx context.Context, s string) ([]Record, error) {
	return []Record{}, nil
}
