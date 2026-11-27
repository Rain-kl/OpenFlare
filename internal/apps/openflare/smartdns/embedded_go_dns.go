// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Rain-kl/Wavelet/internal/apps/openflare/netintel"
	"github.com/Rain-kl/Wavelet/internal/apps/openflare/qualityapi"
	"github.com/Rain-kl/Wavelet/internal/openflare/protocol"
	"github.com/Rain-kl/Wavelet/internal/repository"
	"github.com/Rain-kl/Wavelet/pkg/cache/ram"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/miekg/dns"
)

const (
	embeddedCacheMaxSize   = 8192
	embeddedCacheMaxTTL    = 5 * time.Second
	embeddedRRLLimitPerSec = 1000.0
	embeddedRRLBurst       = 2000.0
)

type cachedDNSAnswer struct {
	answers   []dns.RR
	authority []dns.RR
	rcode     int
	version   uint64
	expiresAt time.Time
}

type rrlBucket struct {
	tokens     float64
	lastRefill time.Time
}

type prefixRRL struct {
	mu      sync.Mutex
	buckets map[string]*rrlBucket
}

func newPrefixRRL() *prefixRRL {
	return &prefixRRL{buckets: make(map[string]*rrlBucket)}
}

func (r *prefixRRL) allow(prefix string, now time.Time) bool {
	if r == nil || prefix == "" {
		return true
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.buckets) > 4096 {
		cutoff := now.Add(-10 * time.Second)
		for k, b := range r.buckets {
			if b.lastRefill.Before(cutoff) {
				delete(r.buckets, k)
			}
		}
	}

	b, ok := r.buckets[prefix]
	if !ok {
		r.buckets[prefix] = &rrlBucket{tokens: embeddedRRLBurst - 1, lastRefill: now}
		return true
	}
	elapsed := now.Sub(b.lastRefill).Seconds()
	if elapsed > 0 {
		b.tokens = minFloat(embeddedRRLBurst, b.tokens+elapsed*embeddedRRLLimitPerSec)
		b.lastRefill = now
	}
	if b.tokens < 1.0 {
		return false
	}
	b.tokens -= 1.0
	return true
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

type dnsMetricsCollector struct {
	queriesTotal       atomic.Uint64
	queriesSuccess     atomic.Uint64
	queriesNXDomain    atomic.Uint64
	queriesServFail    atomic.Uint64
	queriesRateLimited atomic.Uint64
	cacheHits          atomic.Uint64
	cacheLookups       atomic.Uint64

	latMu     sync.Mutex
	latRing   []float64
	lastFlush time.Time
	lastTotal uint64
}

func newDNSMetricsCollector() *dnsMetricsCollector {
	return &dnsMetricsCollector{
		latRing:   make([]float64, 0, 512),
		lastFlush: time.Now().UTC(),
	}
}

func (m *dnsMetricsCollector) recordLatency(d time.Duration) {
	ms := float64(d.Microseconds()) / 1000.0
	m.latMu.Lock()
	if len(m.latRing) >= 512 {
		copy(m.latRing, m.latRing[1:])
		m.latRing[len(m.latRing)-1] = ms
	} else {
		m.latRing = append(m.latRing, ms)
	}
	m.latMu.Unlock()
}

func (m *dnsMetricsCollector) snapshotHeartbeat(snapVer uint64, etag, listenAddr string) protocol.DNSHeartbeatRequest {
	now := time.Now().UTC()
	total := m.queriesTotal.Load()
	success := m.queriesSuccess.Load()
	nxdomain := m.queriesNXDomain.Load()
	servfail := m.queriesServFail.Load()
	ratelimited := m.queriesRateLimited.Load()
	hits := m.cacheHits.Load()
	lookups := m.cacheLookups.Load()

	var hitRatio float64
	if lookups > 0 {
		hitRatio = float64(hits) / float64(lookups)
	}

	m.latMu.Lock()
	elapsed := now.Sub(m.lastFlush).Seconds()
	deltaTotal := total - m.lastTotal
	m.lastFlush = now
	m.lastTotal = total

	samples := make([]float64, len(m.latRing))
	copy(samples, m.latRing)
	m.latRing = m.latRing[:0]
	m.latMu.Unlock()

	var qps float64
	if elapsed > 0 {
		qps = float64(deltaTotal) / elapsed
	}
	p50, p95, p99 := latencyPercentiles(samples)

	return protocol.DNSHeartbeatRequest{
		Version:            "embedded-go",
		ListenAddr:         listenAddr,
		SnapshotVersion:    snapVer,
		SnapshotETag:       etag,
		QPS:                round4(qps),
		P50Ms:              round4(p50),
		P95Ms:              round4(p95),
		P99Ms:              round4(p99),
		QueriesTotal:       total,
		QueriesSuccess:     success,
		QueriesNXDomain:    nxdomain,
		QueriesServFail:    servfail,
		QueriesRateLimited: ratelimited,
		CacheHitRatio:      round4(hitRatio),
		CapturedAt:         &now,
	}
}

func latencyPercentiles(samples []float64) (p50, p95, p99 float64) {
	if len(samples) == 0 {
		return 0, 0, 0
	}
	sort.Float64s(samples)
	pct := func(q float64) float64 {
		idx := int(float64(len(samples)-1) * q)
		if idx < 0 {
			idx = 0
		}
		if idx >= len(samples) {
			idx = len(samples) - 1
		}
		return samples[idx]
	}
	return pct(0.50), pct(0.95), pct(0.99)
}

// EmbeddedGoDNSServer runs a pure-Go authoritative miekg/dns UDP+TCP server fed by SnapshotManager.
type EmbeddedGoDNSServer struct {
	listenAddr string
	snapshots  *SnapshotManager
	cache      *ram.Cache[string, cachedDNSAnswer]
	hysteresis *HysteresisTracker
	rrl        *prefixRRL
	metrics    *dnsMetricsCollector

	mu         sync.Mutex
	udpServer  *dns.Server
	tcpServer  *dns.Server
	udpAddr    string
	tcpAddr    string
	stopCancel context.CancelFunc
}

// NewEmbeddedGoDNSServer constructs an in-process authoritative Go DNS server.
func NewEmbeddedGoDNSServer(listenAddr string, snapshots *SnapshotManager) *EmbeddedGoDNSServer {
	if snapshots == nil {
		snapshots = defaultSnapshotManager
	}
	return &EmbeddedGoDNSServer{
		listenAddr: listenAddr,
		snapshots:  snapshots,
		cache:      ram.MustNew[string, cachedDNSAnswer](ram.Options{MaximumSize: embeddedCacheMaxSize}),
		hysteresis: NewHysteresisTracker(16384),
		rrl:        newPrefixRRL(),
		metrics:    newDNSMetricsCollector(),
	}
}

// Start binds UDP and TCP sockets and begins serving authoritative DNS queries.
func (s *EmbeddedGoDNSServer) Start(ctx context.Context) error {
	mux := dns.NewServeMux()
	mux.HandleFunc(".", s.serveDNS)

	pc, err := net.ListenPacket("udp", s.listenAddr)
	if err != nil {
		return err
	}
	ln, err := net.Listen("tcp", pc.LocalAddr().String())
	if err != nil {
		_ = pc.Close()
		return err
	}

	udpReady := make(chan struct{})
	tcpReady := make(chan struct{})

	udpSrv := &dns.Server{
		PacketConn:        pc,
		Handler:           mux,
		NotifyStartedFunc: func() { close(udpReady) },
	}
	tcpSrv := &dns.Server{
		Listener:          ln,
		Handler:           mux,
		NotifyStartedFunc: func() { close(tcpReady) },
	}

	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	s.mu.Lock()
	s.udpServer = udpSrv
	s.tcpServer = tcpSrv
	s.udpAddr = pc.LocalAddr().String()
	s.tcpAddr = ln.Addr().String()
	s.stopCancel = cancel
	s.mu.Unlock()

	go func() {
		if err := udpSrv.ActivateAndServe(); err != nil {
			logger.WarnF(runCtx, "[SmartDNS-Go] UDP server stopped: %v", err)
		}
	}()
	go func() {
		if err := tcpSrv.ActivateAndServe(); err != nil {
			logger.WarnF(runCtx, "[SmartDNS-Go] TCP server stopped: %v", err)
		}
	}()

	<-udpReady
	<-tcpReady

	go s.runHeartbeatLoop(runCtx)
	return nil
}

// Addr returns the bound UDP listen address (useful when started with port ":0").
func (s *EmbeddedGoDNSServer) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.udpAddr
}

// Stop gracefully shuts down UDP/TCP listeners and heartbeat reporting.
func (s *EmbeddedGoDNSServer) Stop(ctx context.Context) error {
	s.mu.Lock()
	cancel := s.stopCancel
	udpSrv := s.udpServer
	tcpSrv := s.tcpServer
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if udpSrv != nil {
		_ = udpSrv.ShutdownContext(ctx)
	}
	if tcpSrv != nil {
		_ = tcpSrv.ShutdownContext(ctx)
	}
	return nil
}

func (s *EmbeddedGoDNSServer) runHeartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			inst, err := repository.GetDNSInstanceByInstanceID(ctx, embeddedInstanceID)
			if err != nil {
				continue
			}
			var ver uint64
			var etag string
			if bundle := s.snapshots.Current(); bundle != nil {
				ver = bundle.Version()
				etag = bundle.ETag
			}
			hb := s.metrics.snapshotHeartbeat(ver, etag, s.Addr())
			_, _ = RecordDNSHeartbeat(ctx, inst, hb)
		}
	}
}

func (s *EmbeddedGoDNSServer) serveDNS(w dns.ResponseWriter, r *dns.Msg) {
	start := time.Now()
	s.metrics.queriesTotal.Add(1)
	defer func() {
		s.metrics.recordLatency(time.Since(start))
	}()

	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Authoritative = true
	resp.RecursionAvailable = false

	if len(r.Question) == 0 {
		resp.Rcode = dns.RcodeFormatError
		s.metrics.queriesServFail.Add(1)
		_ = w.WriteMsg(resp)
		return
	}

	q := r.Question[0]
	qname := normalizeDomain(q.Name)
	qtypeName := dns.TypeToString[q.Qtype]
	if qtypeName == "" {
		qtypeName = strconv.Itoa(int(q.Qtype))
	}

	bundle, err := s.snapshots.EnsureCurrent(context.Background())
	if err != nil || bundle == nil {
		resp.Rcode = dns.RcodeServerFailure
		s.metrics.queriesServFail.Add(1)
		_ = w.WriteMsg(resp)
		return
	}
	env := &bundle.Envelope

	// Check zone authority when delegation_domain is configured.
	if env.DelegationDomain != "" && !isInAuthoritativeScope(env, qname) {
		resp.Rcode = dns.RcodeRefused
		_ = w.WriteMsg(resp)
		return
	}

	if q.Qtype == dns.TypeSOA {
		resp.Answer = []dns.RR{buildSOARecord(q.Name, env)}
		s.metrics.queriesSuccess.Add(1)
		_ = w.WriteMsg(resp)
		return
	}
	if q.Qtype == dns.TypeNS {
		ttl := uint32(maxInt(env.DefaultTTL, 60))
		for _, nsHost := range effectiveNSHosts(env) {
			resp.Answer = append(resp.Answer, &dns.NS{
				Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeNS, Class: dns.ClassINET, Ttl: ttl},
				Ns:  dns.Fqdn(nsHost),
			})
		}
		s.metrics.queriesSuccess.Add(1)
		_ = w.WriteMsg(resp)
		return
	}

	if q.Qtype != dns.TypeA && q.Qtype != dns.TypeAAAA {
		resp.Ns = []dns.RR{buildSOARecord(zoneApexForQuery(env, q.Name), env)}
		s.metrics.queriesSuccess.Add(1)
		_ = w.WriteMsg(resp)
		return
	}

	clientIP, ecsReq := extractClientIPAndECS(w, r)
	clientPrefix := normalizeClientPrefix(clientIP)
	now := time.Now().UTC()

	if !s.rrl.allow(clientPrefix, now) {
		resp.Rcode = dns.RcodeRefused
		s.metrics.queriesRateLimited.Add(1)
		_ = w.WriteMsg(resp)
		return
	}

	cacheKey := fmt.Sprintf("%d|%s|%s|%s", env.Version, qname, qtypeName, clientPrefix)
	s.metrics.cacheLookups.Add(1)
	if cached, ok := s.cache.GetIfPresent(cacheKey); ok && cached.version == env.Version && now.Before(cached.expiresAt) {
		s.metrics.cacheHits.Add(1)
		resp.Rcode = cached.rcode
		for _, rr := range cached.answers {
			cloned := dns.Copy(rr)
			cloned.Header().Name = q.Name
			resp.Answer = append(resp.Answer, cloned)
		}
		for _, rr := range cached.authority {
			resp.Ns = append(resp.Ns, dns.Copy(rr))
		}
		attachECSScopeReply(resp, ecsReq)
		s.metrics.queriesSuccess.Add(1)
		_ = w.WriteMsg(resp)
		return
	}

	policy, ok := matchPolicyForDomain(env, qname)
	if !ok {
		resp.Rcode = dns.RcodeNameError
		resp.Ns = []dns.RR{buildSOARecord(zoneApexForQuery(env, q.Name), env)}
		s.metrics.queriesNXDomain.Add(1)
		_ = w.WriteMsg(resp)
		return
	}

	profile := qualityapi.ClientProfile{
		ClientIP: clientIP.String(),
		IPFamily: familyForQType(qtypeName),
	}
	if clientIP != nil {
		if lookup := netintel.LookupIP(clientIP.String()); lookup != nil {
			profile.ASN = lookup.ASN
			profile.CountryISO = lookup.CountryISO
			profile.RegionCode = lookup.RegionCode
			profile.ContinentCode = lookup.ContinentCode
			profile.Latitude = lookup.Latitude
			profile.Longitude = lookup.Longitude
			profile.GeoAccuracyKm = lookup.AccuracyRadiusKm
			profile.GeoSource = lookup.GeoSource
			profile.AnycastStatus = lookup.AnycastStatus
		}
	}

	nodeIDs := make([]string, 0, len(env.Nodes))
	for _, n := range env.Nodes {
		nodeIDs = append(nodeIDs, n.NodeID)
	}
	qualities := qualityapi.Default().EvaluateNodes(context.Background(), nodeIDs, profile)

	sel := SelectNodes(SelectInput{
		Domain:     qname,
		QType:      qtypeName,
		ClientKey:  clientPrefix,
		Client:     profile,
		Nodes:      env.Nodes,
		Policy:     policy,
		Quality:    qualities,
		Hysteresis: s.hysteresis,
		Now:        now,
	})

	ttl := uint32(maxInt(sel.TTL, 10))
	answers := make([]dns.RR, 0, len(sel.Answers))
	for _, ans := range sel.Answers {
		parsed := net.ParseIP(ans.IP)
		if parsed == nil {
			continue
		}
		if q.Qtype == dns.TypeA {
			if v4 := parsed.To4(); v4 != nil {
				answers = append(answers, &dns.A{
					Hdr: dns.RR_Header{Name: q.Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: ttl},
					A:   v4,
				})
			}
		} else if q.Qtype == dns.TypeAAAA && parsed.To4() == nil {
			answers = append(answers, &dns.AAAA{
				Hdr:  dns.RR_Header{Name: q.Name, Rrtype: dns.TypeAAAA, Class: dns.ClassINET, Ttl: ttl},
				AAAA: parsed,
			})
		}
	}

	var authority []dns.RR
	if len(answers) == 0 {
		authority = []dns.RR{buildSOARecord(zoneApexForQuery(env, q.Name), env)}
	}
	resp.Answer = answers
	resp.Ns = authority
	attachECSScopeReply(resp, ecsReq)

	cacheTTL := minDuration(time.Duration(ttl)*time.Second, embeddedCacheMaxTTL)
	s.cache.Set(cacheKey, cachedDNSAnswer{
		answers:   cloneRRs(answers),
		authority: cloneRRs(authority),
		rcode:     resp.Rcode,
		version:   env.Version,
		expiresAt: now.Add(cacheTTL),
	})

	s.metrics.queriesSuccess.Add(1)
	_ = w.WriteMsg(resp)
}

func cloneRRs(items []dns.RR) []dns.RR {
	if len(items) == 0 {
		return nil
	}
	out := make([]dns.RR, len(items))
	for i, rr := range items {
		out[i] = dns.Copy(rr)
	}
	return out
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func extractClientIPAndECS(w dns.ResponseWriter, r *dns.Msg) (net.IP, *dns.EDNS0_SUBNET) {
	for _, extra := range r.Extra {
		opt, ok := extra.(*dns.OPT)
		if !ok {
			continue
		}
		for _, o := range opt.Option {
			ecs, ok := o.(*dns.EDNS0_SUBNET)
			if ok && ecs.Address != nil {
				return ecs.Address, ecs
			}
		}
	}
	if addr := w.RemoteAddr(); addr != nil {
		switch a := addr.(type) {
		case *net.UDPAddr:
			if a.IP != nil {
				return a.IP, nil
			}
		case *net.TCPAddr:
			if a.IP != nil {
				return a.IP, nil
			}
		}
	}
	return net.ParseIP("127.0.0.1"), nil
}

func attachECSScopeReply(resp *dns.Msg, reqECS *dns.EDNS0_SUBNET) {
	if reqECS == nil || reqECS.Address == nil {
		return
	}
	scope := uint8(24)
	if reqECS.Family == 2 || reqECS.Address.To4() == nil {
		scope = 48
	}
	if reqECS.SourceNetmask < scope {
		scope = reqECS.SourceNetmask
	}
	replyECS := &dns.EDNS0_SUBNET{
		Code:          dns.EDNS0SUBNET,
		Family:        reqECS.Family,
		SourceNetmask: reqECS.SourceNetmask,
		SourceScope:   scope,
		Address:       reqECS.Address,
	}
	opt := &dns.OPT{
		Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT},
	}
	opt.SetUDPSize(4096)
	opt.Option = append(opt.Option, replyECS)
	resp.Extra = append(resp.Extra, opt)
}

func isInAuthoritativeScope(env *protocol.DNSSnapshotEnvelope, qname string) bool {
	if env == nil || env.DelegationDomain == "" {
		return true
	}
	del := normalizeDomain(env.DelegationDomain)
	if qname == del || strings.HasSuffix(qname, "."+del) {
		return true
	}
	for _, r := range env.DomainRoutes {
		if r.TargetHost == qname {
			return true
		}
		for _, m := range r.MemberDomains {
			if matchDomainPattern(m, qname) {
				return true
			}
		}
	}
	for _, p := range env.Policies {
		if p.ScopeType == "domains" {
			for _, pat := range p.ScopeDomains {
				if matchDomainPattern(pat, qname) {
					return true
				}
			}
		}
	}
	return false
}

func zoneApexForQuery(env *protocol.DNSSnapshotEnvelope, qname string) string {
	if env != nil && env.DelegationDomain != "" {
		return dns.Fqdn(env.DelegationDomain)
	}
	return dns.Fqdn(qname)
}

func effectiveNSHosts(env *protocol.DNSSnapshotEnvelope) []string {
	if env != nil && len(env.NSHosts) > 0 {
		return env.NSHosts
	}
	if env != nil && env.DelegationDomain != "" {
		return []string{"ns1." + env.DelegationDomain}
	}
	return []string{"ns1.localhost"}
}

func buildSOARecord(zone string, env *protocol.DNSSnapshotEnvelope) *dns.SOA {
	fqZone := dns.Fqdn(zone)
	nsHosts := effectiveNSHosts(env)
	primaryNS := dns.Fqdn(nsHosts[0])
	mbox := "hostmaster." + strings.TrimSuffix(fqZone, ".") + "."
	if mbox == "hostmaster.." {
		mbox = "hostmaster.localhost."
	}
	serial := uint32(1)
	minTTL := uint32(60)
	if env != nil {
		if env.Version > 0 {
			serial = uint32(env.Version)
		}
		if env.DefaultTTL > 0 {
			minTTL = uint32(env.DefaultTTL)
		}
	}
	return &dns.SOA{
		Hdr:     dns.RR_Header{Name: fqZone, Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: minTTL},
		Ns:      primaryNS,
		Mbox:    mbox,
		Serial:  serial,
		Refresh: 300,
		Retry:   60,
		Expire:  604800,
		Minttl:  minTTL,
	}
}
