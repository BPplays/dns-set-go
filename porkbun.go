package dns_set

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"strconv"
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
    existingRecChan := make(chan pb_dom_rec)
    pbRecs := make([]pb_dom_rec, 0, len(hostnames))

    for _, hostname := range hostnames {
        wg.Add(1)
        go func(host string) {
            defer wg.Done()
            recs, err := p.self.RetrieveRecords(ctx, host)
            if err != nil {
                return
            }
            existingRecChan <- pb_dom_rec{host, recs}
        }(hostname)
    }

    go func() {
        wg.Wait()
        close(existingRecChan)
    }()

    for rec := range existingRecChan {
        pbRecs = append(pbRecs, rec)
    }

    return pbRecs, nil
}

func (p Porkbun) baseRecToPbRec(r Record) porkbun.Record {
	return porkbun.Record{
		Name: r.Name,
		Type: r.Type,
		Content: r.Content,
		TTL: r.TTL,
		Prio: r.Prio,
		Notes: r.Notes,
	}
}

func (p Porkbun) pbRecToBaseRec(domain string, r porkbun.Record) Record {
	return Record{
		Domain: domain,
		Name: r.Name,
		Type: r.Type,
		Content: r.Content,
		TTL: r.TTL,
		Prio: r.Prio,
		Notes: r.Notes,
	}
}

func (p Porkbun) inSetSingleName(ctx context.Context, domain string, records []Record, existingRecs pb_dom_rec) error {


	recExists := make(map[Record]bool)

	for _, rec := range records {
		recExists[rec] = false
	}
	for _, rec := range existingRecs.records {
		recExists[p.pbRecToBaseRec(existingRecs.domain, rec)] = false
	}


	for _, rec := range records {
		for _, eRec := range existingRecs.records {
			if rec == p.pbRecToBaseRec(rec.Domain, eRec) { recExists[rec] = true; }
		}
	}



	eRecMapCheck := make(map[Record]bool)

	for _, eRec := range existingRecs.records {
		eRecMapCheck[p.pbRecToBaseRec(existingRecs.domain, eRec)] = true
	}

	for _, rec := range records {
		if len(existingRecs.records) >= len(records) {
			break
		}
		pbRec := p.baseRecToPbRec(rec)

		id, err := p.self.CreateRecord(ctx, rec.Domain, pbRec)
		if err != nil {
			return err
		}
		pbRec.ID = strconv.Itoa(id)

		existingRecs.records = append(existingRecs.records, pbRec)
	}


	i := len(existingRecs.records) - 1
	for len(existingRecs.records) > len(records) {
		id, err := strconv.Atoi(existingRecs.records[i].ID)
		if err != nil {
			return err
		}

		err = p.self.DeleteRecord(ctx, existingRecs.domain, id)
		if err != nil {
			return err
		}
		existingRecs.records = append(existingRecs.records[:i], existingRecs.records[i+1:]...)
		i++

	}

	for _, rec := range records {
		for _, eRec := range existingRecs.records {
			if !recExists[rec] {
				id, err := strconv.Atoi(existingRecs.records[i].ID)
				if err != nil { return err }

				p.self.EditRecord(ctx, domain, id, eRec)
			}
		}

	}

	return nil
}

func (p Porkbun) inSetDns(ctx context.Context, records []Record) error {
	existingRecs, err := p.inGetDns(ctx, recordsToHostnames(records))
	if err != nil {
		log.Println("error", err)
	}


	existingRecMap := make(map[string]*[]porkbun.Record)

	for _, eRec := range existingRecs {
		(*existingRecMap[eRec.domain]) = append((*existingRecMap[eRec.domain]), eRec.records...)
	}

	recMap := make(map[string]*[]Record)

	for _, rec := range records {
		(*recMap[rec.Domain]) = append((*recMap[rec.Domain]), rec)
	}



	var domains []string
	for _, rec := range records {
		domains = append(domains, rec.Domain)
	}


	var wg sync.WaitGroup

	for _, domain := range domains {
		wg.Add(1)
		go func(ctx context.Context, domain string, records []Record, pbRecs []porkbun.Record) {
			defer wg.Done()
			p.inSetSingleName(
				ctx,
				domain,
				records,
				pb_dom_rec{domain: domain, records: pbRecs},
			)


		}(ctx, domain, (*recMap[domain]), (*existingRecMap[domain]))
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


func (p Porkbun) GetSuppoertedRecords() ([]string) {
	return []string{
		"AAAA",
		"A",
		"MX",
		"CNAME",
		"ALIAS",
		"TXT",
		"NS",
		"SRV",
		"TLSA",
		"CAA",
	}
}
