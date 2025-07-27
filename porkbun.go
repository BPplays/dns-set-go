package dns_set

import (
	"context"
	"fmt"
	"net/netip"
	"log"
	"sync"

	"github.com/nrdcg/porkbun"
	"github.com/seancfoley/ipaddress-go/ipaddr"
)

type Porkbun struct {
	self *porkbun.Client
}

type pb_dom_rec struct {
	domain string
	records []porkbun.Record
}

func (p Porkbun) SetAuth(a Auth) DnsAPI {

	p.self = porkbun.New(a.ApiSecretKey, a.ApiKey)
	return p
}

func IPv6ToReverseDNS(ip netip.Prefix) string {
	exp, err := ipaddr.NewIPAddressFromNetNetIPPrefix(ip)
	if err != nil {
		return ""
	}

	revdns, err := exp.GetSection().ToReverseDNSString()
	if err != nil {
		log.Fatalln(err)
	}

	return revdns
}

func recordsToHostnames(r []Record) (hostnames []string) {
	for _, rec := range r {
		hostnames = append(hostnames, rec.Domain)
	}
	return hostnames
}

func (p Porkbun) inGetDns(ctx context.Context, hostnames []string) ([]pb_dom_rec, error) {
	var wg sync.WaitGroup
	var pbRecs []pb_dom_rec
	existingRecChan := make(chan pb_dom_rec)

	for _, hostname := range hostnames {
		wg.Add(1)
		go func(host string) {
			defer wg.Done()
			rec, err := p.self.RetrieveRecords(ctx, hostname)
			if err != nil {
				return
			}
			existingRecChan <- pb_dom_rec{hostname, rec}

		}(hostname)
	}
	wg.Wait()
	// for _, rec := range <-existingRecChan {
	for rec := range existingRecChan {
		pbRecs = append(pbRecs, rec)
	}

	return pbRecs, nil
}

func (p Porkbun) inSetSingleName(ctx context.Context, name string, records []Record, existingRecs pb_dom_rec) error {

	for len(existingRecs) > len(records) {
		p.self.DeleteRecord(ctx, existingRecs.domain, existingRecs.records[0].ID)
	}
	p.self.CreateRecord()
}

func (p Porkbun) inSetDns(ctx context.Context, records []Record) error {
	existingRecs, err := p.inGetDns(ctx, recordsToHostnames(records))
	if err != nil {
		log.Println("error", err)
	}


	eRecMap := make(map[string]*[]porkbun.Record)

	for _, eRec := range existingRecs {
		(*eRecMap[eRec.Name]) = append((*eRecMap[eRec.Name]), eRec)
	}


	var wg sync.WaitGroup

	for _, record := range records {
		wg.Add(1)
		go func() {
			defer wg.Done()


		}()
	}
	wg.Wait()

	return nil

}

func (p Porkbun) SetDns(ctx context.Context, records []Record) error {
	select {
	case <-ctx.Done():
		fmt.Println("Task cancelled")
		return ctx.Err()
	default:
		for i, _ := range records {
			records[i].SetDefaults()
		}
		return p.inSetDns(ctx, records)
	}
}

func (p Porkbun) GetDns(ctx context.Context, hostnames []string) ([]Record, error) {
	select {
	case <-ctx.Done():
		fmt.Println("Task cancelled")
		return []Record{}, ctx.Err()
	default:
		return p.inGetDns(ctx, hostnames)
	}
}
