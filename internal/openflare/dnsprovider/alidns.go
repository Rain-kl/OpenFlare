// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dnsprovider

import (
	"context"
	"errors"
	"net/http"

	"github.com/Rain-kl/Wavelet/pkg/httppool"

	ali "github.com/alibabacloud-go/alidns-20150109/v5/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"
)

const (
	aliPageSize        int64 = 100
	aliEndpoint              = "alidns.aliyuncs.com"
	aliDefaultLine           = "default"
	errAliRecordResult       = "DNS record response from Alibaba Cloud is empty"
)

type aliDNS struct{ client *ali.Client }

func newAliDNS(c credentials) (Adapter, error) {
	cfg := &openapi.Config{AccessKeyId: &c.AccessKeyID, AccessKeySecret: &c.AccessKeySecret, Endpoint: dara.String(aliEndpoint)}
	client, err := ali.NewClient(cfg)
	if err != nil {
		return nil, err
	}
	return &aliDNS{client}, nil
}

type aliHTTP struct{ ctx context.Context }

var aliHTTPClient = httppool.NewClient(defaultHTTPTimeout)

func (h aliHTTP) Call(request *http.Request, _ *http.Transport) (*http.Response, error) {
	if request.URL.Scheme != "https" || request.URL.Host != aliEndpoint {
		return nil, errors.New("unexpected Alibaba Cloud DNS API endpoint")
	}
	return aliHTTPClient.Do(request.WithContext(h.ctx)) //nolint:gosec // The SDK URL is restricted to the fixed HTTPS API endpoint above.
}

func (a *aliDNS) withContext(ctx context.Context) *ali.Client {
	client := *a.client
	client.HttpClient = aliHTTP{ctx}
	return &client
}

func (a *aliDNS) VerifyToken(ctx context.Context) error {
	_, err := a.ListZones(ctx)
	return err
}

func (a *aliDNS) ListZones(ctx context.Context) ([]Zone, error) {
	var zones []Zone
	for page := int64(1); ; page++ {
		req := new(ali.DescribeDomainsRequest).SetPageNumber(page).SetPageSize(aliPageSize)
		resp, err := a.withContext(ctx).DescribeDomainsWithOptions(req, &dara.RuntimeOptions{})
		if err != nil {
			return nil, err
		}
		if resp.Body == nil || resp.Body.Domains == nil {
			return zones, nil
		}
		rows := resp.Body.Domains.Domain
		for _, row := range rows {
			name := dnsName(value(row.PunyCode))
			if name == "" {
				name = dnsName(value(row.DomainName))
			}
			if name != "" {
				zones = append(zones, Zone{ID: name, Name: name})
			}
		}
		if len(rows) == 0 || page*aliPageSize >= value(resp.Body.TotalCount) {
			return zones, nil
		}
	}
}

func (a *aliDNS) FindZone(ctx context.Context, name string) (*Zone, error) {
	return findExactZone(ctx, a, name)
}

func (a *aliDNS) GetRecord(ctx context.Context, zone, id string) (*DNSRecord, error) {
	req := new(ali.DescribeDomainRecordInfoRequest).SetRecordId(id)
	resp, err := a.withContext(ctx).DescribeDomainRecordInfoWithOptions(req, &dara.RuntimeOptions{})
	if err != nil {
		return nil, err
	}
	r := resp.Body
	if r == nil || (dnsName(value(r.DomainName)) != dnsName(zone) && dnsName(value(r.PunyCode)) != dnsName(zone)) {
		return nil, errors.New("record is outside the Alibaba Cloud DNS zone")
	}
	return &DNSRecord{ID: value(r.RecordId), Type: value(r.Type), Name: fullName(zone, value(r.RR)), Content: value(r.Value), TTL: int(value(r.TTL))}, nil
}

func (a *aliDNS) ListRecords(ctx context.Context, zone, name, kind string) ([]DNSRecord, error) {
	rr, err := relativeName(zone, name)
	if err != nil {
		return nil, err
	}
	var records []DNSRecord
	for page := int64(1); ; page++ {
		req := new(ali.DescribeDomainRecordsRequest).SetDomainName(zone).SetRRKeyWord(rr).SetSearchMode("COMBINATION").SetLine(aliDefaultLine).SetPageNumber(page).SetPageSize(aliPageSize)
		if kind != "" {
			req.SetType(kind)
		}
		resp, err := a.withContext(ctx).DescribeDomainRecordsWithOptions(req, &dara.RuntimeOptions{})
		if err != nil {
			return nil, err
		}
		if resp.Body == nil || resp.Body.DomainRecords == nil {
			return records, nil
		}
		rows := resp.Body.DomainRecords.Record
		for _, r := range rows {
			if dnsName(value(r.RR)) != rr || (kind != "" && value(r.Type) != kind) || (value(r.Line) != "" && value(r.Line) != aliDefaultLine) {
				continue
			}
			records = append(records, DNSRecord{ID: value(r.RecordId), Type: value(r.Type), Name: fullName(zone, value(r.RR)), Content: value(r.Value), TTL: int(value(r.TTL))})
		}
		if len(rows) == 0 || page*aliPageSize >= value(resp.Body.TotalCount) {
			return records, nil
		}
	}
}

func (a *aliDNS) CreateRecord(ctx context.Context, zone string, in RecordInput) (*DNSRecord, error) {
	rr, err := relativeName(zone, in.Name)
	if err != nil {
		return nil, err
	}
	req := new(ali.AddDomainRecordRequest).SetDomainName(zone).SetRR(rr).SetLine(aliDefaultLine).SetType(in.Type).SetValue(in.Content).SetTTL(int64(recordTTL(in.TTL)))
	resp, err := a.withContext(ctx).AddDomainRecordWithOptions(req, &dara.RuntimeOptions{})
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, errors.New(errAliRecordResult)
	}
	return writtenRecord(value(resp.Body.RecordId), in)
}

func (a *aliDNS) UpdateRecord(ctx context.Context, zone, id string, in RecordInput) (*DNSRecord, error) {
	rr, err := relativeName(zone, in.Name)
	if err != nil {
		return nil, err
	}
	req := new(ali.UpdateDomainRecordRequest).SetRecordId(id).SetRR(rr).SetLine(aliDefaultLine).SetType(in.Type).SetValue(in.Content).SetTTL(int64(recordTTL(in.TTL)))
	resp, err := a.withContext(ctx).UpdateDomainRecordWithOptions(req, &dara.RuntimeOptions{})
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, errors.New(errAliRecordResult)
	}
	return writtenRecord(value(resp.Body.RecordId), in)
}

func (a *aliDNS) DeleteRecord(ctx context.Context, _, id string) error {
	_, err := a.withContext(ctx).DeleteDomainRecordWithOptions(new(ali.DeleteDomainRecordRequest).SetRecordId(id), &dara.RuntimeOptions{})
	return err
}
