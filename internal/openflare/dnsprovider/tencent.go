// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package dnsprovider

import (
	"context"
	"errors"
	"strconv"

	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	tcerror "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/errors"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	dnspod "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/dnspod/v20210323"
)

const (
	tencentPageSize = 100
	dnsPodMaxTTL    = 604800
)
const defaultDNSLine = "默认"

type tencentDNS struct{ client *dnspod.Client }

func newTencentDNS(c credentials) (Adapter, error) {
	cfg := profile.NewClientProfile()
	cfg.HttpProfile.Endpoint = "dnspod.tencentcloudapi.com"
	cfg.HttpProfile.ReqTimeout = 20
	client, err := dnspod.NewClient(common.NewCredential(c.SecretID, c.SecretKey), "", cfg)
	if err != nil {
		return nil, err
	}
	return &tencentDNS{client}, nil
}

func (t *tencentDNS) VerifyToken(ctx context.Context) error {
	_, err := t.ListZones(ctx)
	return err
}

func (t *tencentDNS) ListZones(ctx context.Context) ([]Zone, error) {
	var zones []Zone
	for offset := int64(0); ; {
		req := dnspod.NewDescribeDomainListRequest()
		req.Offset, req.Limit = &offset, common.Int64Ptr(tencentPageSize)
		resp, err := t.client.DescribeDomainListWithContext(ctx, req)
		if tencentNoData(err, dnspod.RESOURCENOTFOUND_NODATAOFDOMAIN) {
			return zones, nil
		}
		if err != nil {
			return nil, err
		}
		if resp.Response == nil {
			return nil, errors.New("DNSPod returned no domain list")
		}
		rows := resp.Response.DomainList
		for _, row := range rows {
			name := dnsName(value(row.Punycode))
			if name == "" {
				name = dnsName(value(row.Name))
			}
			if name != "" {
				zones = append(zones, Zone{ID: name, Name: name})
			}
		}
		offset += int64(len(rows))
		if len(rows) < tencentPageSize {
			return zones, nil
		}
	}
}

func (t *tencentDNS) FindZone(ctx context.Context, name string) (*Zone, error) {
	return findExactZone(ctx, t, name)
}

func (t *tencentDNS) GetRecord(ctx context.Context, zone, id string) (*DNSRecord, error) {
	recordID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, err
	}
	req := dnspod.NewDescribeRecordRequest()
	req.Domain, req.RecordId = &zone, &recordID
	resp, err := t.client.DescribeRecordWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Response == nil || resp.Response.RecordInfo == nil {
		return nil, errors.New("DNSPod returned no record")
	}
	r := resp.Response.RecordInfo
	return &DNSRecord{ID: id, Type: value(r.RecordType), Name: fullName(zone, value(r.SubDomain)), Content: value(r.Value), TTL: dnsPodTTL(value(r.TTL))}, nil
}

func (t *tencentDNS) ListRecords(ctx context.Context, zone, name, kind string) ([]DNSRecord, error) {
	rr, err := relativeName(zone, name)
	if err != nil {
		return nil, err
	}
	var records []DNSRecord
	for offset := uint64(0); ; {
		req := dnspod.NewDescribeRecordListRequest()
		req.Domain, req.Subdomain, req.RecordLine = &zone, &rr, common.StringPtr(defaultDNSLine)
		req.Offset, req.Limit = &offset, common.Uint64Ptr(tencentPageSize)
		if kind != "" {
			req.RecordType = &kind
		}
		resp, err := t.client.DescribeRecordListWithContext(ctx, req)
		if tencentNoData(err, dnspod.RESOURCENOTFOUND_NODATAOFRECORD) {
			return records, nil
		}
		if err != nil {
			return nil, err
		}
		if resp.Response == nil {
			return nil, errors.New("DNSPod returned no record list")
		}
		rows := resp.Response.RecordList
		for _, r := range rows {
			if dnsName(value(r.Name)) != rr || (kind != "" && value(r.Type) != kind) {
				continue
			}
			records = append(records, DNSRecord{ID: strconv.FormatUint(value(r.RecordId), 10), Type: value(r.Type), Name: fullName(zone, value(r.Name)), Content: value(r.Value), TTL: dnsPodTTL(value(r.TTL))})
		}
		offset += uint64(len(rows))
		if len(rows) < tencentPageSize {
			return records, nil
		}
	}
}

func (t *tencentDNS) CreateRecord(ctx context.Context, zone string, in RecordInput) (*DNSRecord, error) {
	rr, err := relativeName(zone, in.Name)
	if err != nil {
		return nil, err
	}
	req := dnspod.NewCreateRecordRequest()
	req.Domain, req.SubDomain, req.RecordType, req.Value = &zone, &rr, &in.Type, &in.Content
	req.RecordLine, req.TTL = common.StringPtr(defaultDNSLine), common.Uint64Ptr(dnsPodWriteTTL(in.TTL))
	resp, err := t.client.CreateRecordWithContext(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Response == nil || value(resp.Response.RecordId) == 0 {
		return nil, errors.New("DNSPod returned no record identifier")
	}
	return writtenRecord(strconv.FormatUint(value(resp.Response.RecordId), 10), in)
}

func (t *tencentDNS) UpdateRecord(ctx context.Context, zone, id string, in RecordInput) (*DNSRecord, error) {
	rr, err := relativeName(zone, in.Name)
	if err != nil {
		return nil, err
	}
	recordID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return nil, err
	}
	req := dnspod.NewModifyRecordRequest()
	req.Domain, req.SubDomain, req.RecordType, req.Value = &zone, &rr, &in.Type, &in.Content
	req.RecordId, req.RecordLine, req.TTL = &recordID, common.StringPtr(defaultDNSLine), common.Uint64Ptr(dnsPodWriteTTL(in.TTL))
	if _, err = t.client.ModifyRecordWithContext(ctx, req); err != nil {
		return nil, err
	}
	return writtenRecord(id, in)
}

func (t *tencentDNS) DeleteRecord(ctx context.Context, zone, id string) error {
	recordID, err := strconv.ParseUint(id, 10, 64)
	if err != nil {
		return err
	}
	req := dnspod.NewDeleteRecordRequest()
	req.Domain, req.RecordId = &zone, &recordID
	_, err = t.client.DeleteRecordWithContext(ctx, req)
	return err
}

func tencentNoData(err error, code string) bool {
	var sdkErr *tcerror.TencentCloudSDKError
	return errors.As(err, &sdkErr) && sdkErr.Code == code
}

func dnsPodTTL(ttl uint64) int {
	if ttl > dnsPodMaxTTL {
		return dnsPodMaxTTL
	}
	return int(ttl)
}

func dnsPodWriteTTL(ttl int) uint64 {
	if ttl <= 1 {
		return defaultProviderTTL
	}
	return uint64(ttl)
}
