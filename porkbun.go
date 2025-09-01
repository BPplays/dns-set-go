package dns_set

import (
	"context"
	"errors"
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

type action struct {
	action string
	domain DomainSub
	record porkbun.Record
}

func (self action) SetAction(a string) error {
	switch a {
	case "create":
		self.action = "create"
		return nil
	case "edit":
		self.action = "edit"
		return nil
	case "delete":
		self.action = "delete"
		return nil
	case "none":
		self.action = "none"
		return nil
	}
	return errors.ErrUnsupported
}

type actionMap map[string][]action

func (self actionMap) init() {
	self["create"] = []action{}
	self["edit"] = []action{}
	self["delete"] = []action{}
	self["none"] = []action{}
}

func (self actionMap) containsID(s string) bool {
	for _, acts := range self {
		for _, act := range acts {
			if act.record.ID == s { return true }
		}
	}
	return false
}

func (self actionMap) containsRecord(r Record) bool {
	for _, acts := range self {
		for _, act := range acts {
			eRec := pbRecToBaseRec(act.domain, act.record)
			if eRec == r { return true }
		}
	}
	return false
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
		domains = append(domains, rec.Domain)
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

			finalRecs := []porkbun.Record{}
			for _, rec := range recs {
				if rec.Name != domain.full() { continue }
				finalRecs = append(finalRecs, rec)
			}

			if len(finalRecs) > 0 {
				existingRecChan <- pb_dom_rec{domain, finalRecs}
			}

        }(p, domain)
    }

    go func() {
        wg.Wait()
        close(existingRecChan)
    }()

    for domRec := range existingRecChan {
        pbRecs = append(pbRecs, domRec)
	}


    return pbRecs, nil
}

func baseRecToPbRec(r Record) porkbun.Record {
	return porkbun.Record{
		Name: r.Domain.Sub,
		Type: r.Type,
		Content: r.Content,
		TTL: r.TTL,
		Prio: r.Prio,
		Notes: r.Notes,
	}
}

func pbRecToBaseRec(domain DomainSub, r porkbun.Record) Record {
	rec := Record{
		Domain: domain,
		Type: r.Type,
		Content: r.Content,
		TTL: r.TTL,
		Prio: r.Prio,
		Notes: r.Notes,
	}
	rec.SetDefaults()
	return rec
}

func (p Porkbun) getActions(
	ctx context.Context,
	domain DomainSub,
	records []Record,
	existingRecs []pb_dom_rec,
) actionMap {
	actMap := make(actionMap)
	actMap.init()

	for _, rec := range records {
		for _, eRecs := range existingRecs {
			for _, eRec := range eRecs.records {
				eRecBase := pbRecToBaseRec(eRecs.domain, eRec)
				if rec == eRecBase {
					actMap["none"] = append(
						actMap["none"],
						action{action: "none", domain: eRecs.domain, record: eRec},
					)
				}
			}
		}
	}


	existingRecsLen := 0
	for _, eRecs := range existingRecs { for range eRecs.records { existingRecsLen += 1 } }

	recDeficit := len(records) - existingRecsLen

	if recDeficit > 0 {
		for _, rec := range records {
			if recDeficit == 0 { break }

			recDeficit -= 1
			actMap["create"] = append(
				actMap["create"],
				action{action: "create", domain: rec.Domain, record: baseRecToPbRec(rec)},
				)
		}
	}


	return actMap
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
			if rec == pbRecToBaseRec(rec.Domain, eRec) { recExists[rec] = true; }
		}
	}

	fmt.Println()
	log.Println("recEx", recExists)
	fmt.Println()
	log.Println("recExisting", existingRecs.records)
	fmt.Println()



	eRecMapCheck := make(map[Record]bool)

	for _, eRec := range existingRecs.records {
		eRecMapCheck[pbRecToBaseRec(existingRecs.domain, eRec)] = true
	}

	log.Println("existing records:", existingRecs.records)

	for _, rec := range records {
		if len(existingRecs.records) >= len(records) {
			break
		}
		pbRec := baseRecToPbRec(rec)

		log.Printf("making record: %v, %v\n", rec.Domain, pbRec)
		id, err := p.self.CreateRecord(ctx, rec.Domain.Domain, pbRec)
		if err != nil {
			log.Println(err)
			return err
		}
		pbRec.ID = strconv.Itoa(id)

		existingRecs.records = append(existingRecs.records, pbRec)
	}


	for i, rec := range existingRecs.records {
		if len(existingRecs.records) <= len(records) {
			break
		}
		id, err := strconv.Atoi(rec.ID)
		if err != nil {
			return err
		}

		log.Println("deleting", rec)
		err = p.self.DeleteRecord(ctx, existingRecs.domain.Domain, id)
		if err != nil {
			return err
		}
		existingRecs.records = append(existingRecs.records[:i], existingRecs.records[i+1:]...)

	}

	for _, rec := range records {
		for i, eRec := range existingRecs.records {
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

	// for _, rec := range existingRecs {
	//
	// }


	log.Println("all pbdomrecs:", existingRecs)


	existingRecMap := make(map[string][]porkbun.Record)

	for _, eRec := range existingRecs {
		existingRecMap[eRec.domain.full()] = append(existingRecMap[eRec.domain.full()], eRec.records...)
	}

	recMap := make(map[string][]Record)

	for _, rec := range records {
		recMap[rec.Domain.full()] = append(recMap[rec.Domain.full()], rec)
	}



	var domains []DomainSub
	for _, rec := range records {
		if slices.Contains(domains, rec.Domain) { continue }
		domains = append(domains, rec.Domain)
	}


	var wg sync.WaitGroup

	for _, domain := range domains {
		wg.Add(1)
		go func(ctx context.Context, domain DomainSub, records []Record, pbRecs []porkbun.Record) {
			defer wg.Done()
			p.inSetSingleName(
				ctx,
				domain.Domain,
				records,
				pb_dom_rec{domain: domain, records: pbRecs},
			)


		}(ctx, domain, recMap[domain.full()], existingRecMap[domain.full()])
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
		for i := range records {
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
				recs = append(recs, pbRecToBaseRec(pdrec.domain, prec))
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
