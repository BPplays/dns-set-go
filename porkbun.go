package dns_set

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"slices"
	"strconv"
	"sync"

	"github.com/nrdcg/porkbun"
	"github.com/seancfoley/ipaddress-go/ipaddr"
)

type Porkbun struct {
	self *porkbun.Client
}

type pb_dom_rec struct {
	domain DomainSub
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

func recordsToDomains(r []Record) (domains []DomainSub) {
	for _, rec := range r {
		domains = append(domains, DomainSub{rec.Domain, rec.Subdomain})
	}
	return domains
}

func (p Porkbun) inGetDns(ctx context.Context, domains []DomainSub) ([]pb_dom_rec, error) {
    var wg sync.WaitGroup
    existingRecChan := make(chan pb_dom_rec)
    pbRecs := make([]pb_dom_rec, 0, len(domains))

    for _, domain := range domains {
        wg.Add(1)
        go func(p Porkbun, domain DomainSub) {
            defer wg.Done()
            recs, err := p.self.RetrieveRecords(ctx, domain.Domain)
            if err != nil {
                return
            }

            existingRecChan <- pb_dom_rec{domain.full(), recs}
        }(p, domain)
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
		Name: r.Subdomain,
		Type: r.Type,
		Content: r.Content,
		TTL: r.TTL,
		Prio: r.Prio,
		Notes: r.Notes,
	}
}

func (p Porkbun) pbRecToBaseRec(domain string, r porkbun.Record) Record {
	rec := Record{
		Domain: domain,
		Subdomain: r.Name,
		Type: r.Type,
		Content: r.Content,
		TTL: r.TTL,
		Prio: r.Prio,
		Notes: r.Notes,
	}
	rec.SetDefaults()
	return rec
}

func (p Porkbun) inSetSingleName(ctx context.Context, domain string, records []Record, existingRecs pb_dom_rec) error {


	recExists := make(map[Record]bool)

	for _, rec := range records {
		recExists[rec] = false
	}
	// for _, rec := range existingRecs.records {
	// 	recExists[p.pbRecToBaseRec(existingRecs.domain, rec)] = false
	// }


	for _, rec := range records {
		for _, eRec := range existingRecs.records {
			if rec == p.pbRecToBaseRec(rec.Domain, eRec) { recExists[rec] = true; }
		}
	}

	fmt.Println()
	log.Println("recEx", recExists)
	fmt.Println()
	log.Println("recExisting", existingRecs.records)
	fmt.Println()



	eRecMapCheck := make(map[Record]bool)

	for _, eRec := range existingRecs.records {
		eRecMapCheck[p.pbRecToBaseRec(existingRecs.domain, eRec)] = true
	}

	log.Println("existing records:", existingRecs.records)

	for _, rec := range records {
		if len(existingRecs.records) >= len(records) {
			break
		}
		pbRec := p.baseRecToPbRec(rec)

		log.Printf("making record: %v, %v\n", rec.Domain, pbRec)
		id, err := p.self.CreateRecord(ctx, rec.Domain, pbRec)
		if err != nil {
			log.Println(err)
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

		log.Println("deleting")
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

				log.Println("editing record")
				p.self.EditRecord(ctx, domain, id, eRec)
			}
		}

	}

	return nil
}

func (p Porkbun) inSetDns(ctx context.Context, records []Record) error {
	existingRecs, err := p.inGetDns(ctx, recordsToDomains(records))
	if err != nil {
		log.Println("error", err)
	}


	log.Println("all pbdomrecs:", existingRecs)


	existingRecMap := make(map[string][]porkbun.Record)

	for _, eRec := range existingRecs {
		existingRecMap[eRec.domain] = append(existingRecMap[eRec.domain], eRec.records...)
	}

	recMap := make(map[string][]Record)

	for _, rec := range records {
		recMap[rec.Domain] = append(recMap[rec.Domain], rec)
	}



	var domains []DomainSub
	for _, rec := range records {
		if slices.Contains(domains, DomainSub{Domain: rec.Domain, Sub: rec.Subdomain}) { continue }
		domains = append(domains, DomainSub{Domain: rec.Domain, Sub: rec.Subdomain})
	}


	var wg sync.WaitGroup

	for _, domain := range domains {
		wg.Add(1)
		go func(ctx context.Context, domain DomainSub, records []Record, pbRecs []porkbun.Record) {
			defer wg.Done()
			p.inSetSingleName(
				ctx,
				domain,
				records,
				pb_dom_rec{domain: domain, records: pbRecs},
			)


		}(ctx, domain, recMap[domain], existingRecMap[domain])
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

func (p Porkbun) GetDns(ctx context.Context, domains []DomainSub) ([]Record, error) {
	select {
	case <-ctx.Done():
		fmt.Println("Task cancelled")
		return []Record{}, ctx.Err()
	default:
		pbRecs, err := p.inGetDns(ctx, domains)
		if err != nil { return []Record{}, err }

		var recs []Record
		for _, pdrec := range pbRecs {
			for _, prec := range pdrec.records {
				recs = append(recs, p.pbRecToBaseRec(pdrec.domain, prec))
			}
		}

		return recs, nil
	}
}


func (p Porkbun) GetSuppoertedRecords() ([]string, error) {
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
	}, nil
}
