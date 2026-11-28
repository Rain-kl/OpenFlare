// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package netintel

import (
	"bufio"
	"compress/bzip2"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	maxASBFSDepth         = 6
	caidaScannerInitBytes = 64 * 1024
	caidaScannerMaxBytes  = 1024 * 1024
	caidaBFSInitialCap    = 64
)

type caidaGraph struct {
	nodes     map[uint32]uint32
	offsets   []int
	peers     []uint32
	relations []caidaRelation
}

type caidaRelation uint8

const (
	caidaCustomer caidaRelation = iota
	caidaProvider
	caidaPeer
)

var caidaRelationNames = [...]string{RelationCustomer, RelationProvider, RelationPeer}

type caidaEdge struct {
	from, to uint32
	relation caidaRelation
}

func (g *caidaGraph) neighborBounds(asn uint32) (int, int) {
	if g == nil {
		return 0, 0
	}
	if id, ok := g.nodes[asn]; ok {
		return g.offsets[id], g.offsets[id+1]
	}
	return 0, 0
}

func (g *caidaGraph) degree(asn uint32) int {
	start, end := g.neighborBounds(asn)
	return end - start
}

func caidaNodeID(nodes map[uint32]uint32, degrees *[]int, asn uint32) uint32 {
	id, ok := nodes[asn]
	if !ok {
		id = uint32(len(*degrees)) //nolint:gosec // nonzero uint32 ASNs limit the number of distinct nodes
		nodes[asn] = id
		*degrees = append(*degrees, 0)
	}
	(*degrees)[id]++
	return id
}

func loadCAIDAGraph(dir string, manifest *Manifest) (*caidaGraph, DatasetFileStatus) {
	graph := &caidaGraph{}
	status := DatasetFileStatus{
		Key:       "caida_as_rel",
		Filename:  "caida-as-rel2.txt.bz2",
		SourceURL: "https://publicdata.caida.org/datasets/as-relationships/serial-2/",
		License:   "CAIDA Public AUA (Runtime-Only, Citation Required)",
	}
	if strings.TrimSpace(dir) == "" {
		return graph, status
	}
	applyCAIDAManifest(&status, manifest)

	chosenPath := findCAIDAFile(dir, &status)
	if chosenPath == "" {
		return graph, status
	}

	f, err := os.Open(chosenPath) //nolint:gosec // path is resolved inside configured netintel directory
	if err != nil {
		status.Error = err.Error()
		return graph, status
	}
	defer func() { _ = f.Close() }()

	edges, err := parseCAIDAEdges(graph, chosenPath, f)
	if err != nil {
		status.Error = fmt.Sprintf("scan caida as-rel: %v", err)
		return graph, status
	}
	status.Loaded = true
	status.RecordCount = edges
	return graph, status
}

func applyCAIDAManifest(status *DatasetFileStatus, manifest *Manifest) {
	if manifest == nil {
		return
	}
	entry, ok := manifest.Datasets["caida_as_rel"]
	if !ok || entry.Filename == "" {
		return
	}
	status.Filename = entry.Filename
	if entry.SourceURL != "" {
		status.SourceURL = entry.SourceURL
	}
}

func findCAIDAFile(dir string, status *DatasetFileStatus) string {
	candidates := []string{status.Filename, "caida-as-rel2.txt.bz2", "caida-as-rel.txt.bz2", "caida-as-rel2.txt"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		p := filepath.Join(dir, c)
		info, err := os.Stat(p)
		if err != nil || info.IsDir() || info.Size() == 0 {
			continue
		}
		status.Filename = c
		status.Path = p
		status.SizeBytes = info.Size()
		t := info.ModTime().UTC()
		status.UpdatedAt = &t
		return p
	}
	return ""
}

func parseCAIDAEdges(graph *caidaGraph, chosenPath string, f io.Reader) (int, error) {
	r := f
	if strings.HasSuffix(strings.ToLower(chosenPath), ".bz2") {
		r = bzip2.NewReader(f)
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, caidaScannerInitBytes), caidaScannerMaxBytes)
	graph.nodes = make(map[uint32]uint32)
	var degrees []int
	var edges []caidaEdge
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		left, rest, found := strings.Cut(line, "|")
		if !found {
			continue
		}
		right, rest, found := strings.Cut(rest, "|")
		if !found {
			continue
		}
		kind, _, _ := strings.Cut(rest, "|")
		as1, ok1 := parseASNNumber(left)
		as2, ok2 := parseASNNumber(right)
		rel, err := strconv.Atoi(strings.TrimSpace(kind))
		if !ok1 || !ok2 || err != nil || as1 == 0 || as2 == 0 || as1 == as2 {
			continue
		}
		var relation caidaRelation
		switch rel {
		case -1:
			relation = caidaCustomer
		case 0:
			relation = caidaPeer
		default:
			continue
		}
		edges = append(edges, caidaEdge{
			from:     caidaNodeID(graph.nodes, &degrees, as1),
			to:       caidaNodeID(graph.nodes, &degrees, as2),
			relation: relation,
		})
	}

	graph.offsets = make([]int, len(degrees)+1)
	for i, degree := range degrees {
		graph.offsets[i+1] = graph.offsets[i] + degree
	}
	asns := make([]uint32, len(degrees))
	for asn, id := range graph.nodes {
		asns[id] = asn
	}
	graph.peers = make([]uint32, graph.offsets[len(degrees)])
	graph.relations = make([]caidaRelation, len(graph.peers))
	// Reuse the degree counts as write cursors and keep source order within each ASN.
	copy(degrees, graph.offsets)
	for _, edge := range edges {
		from, to := degrees[edge.from], degrees[edge.to]
		graph.peers[from] = asns[edge.to]
		graph.peers[to] = asns[edge.from]
		graph.relations[from], graph.relations[to] = edge.relation, caidaPeer
		if edge.relation == caidaCustomer {
			graph.relations[to] = caidaProvider
		}
		degrees[edge.from]++
		degrees[edge.to]++
	}
	return len(edges), scanner.Err()
}

func (g *caidaGraph) estimatePath(srcASN, dstASN uint32, extraAdj map[uint32][]uint32) (int, []uint32, bool) {
	if srcASN == 0 || dstASN == 0 {
		return 0, nil, false
	}
	if srcASN == dstASN {
		return 0, []uint32{srcASN}, true
	}

	type queueItem struct {
		asn   uint32
		depth int
	}
	parent := make(map[uint32]uint32, caidaBFSInitialCap)
	visited := map[uint32]bool{srcASN: true}
	queue := []queueItem{{asn: srcASN, depth: 0}}

	for head := 0; head < len(queue); head++ {
		cur := queue[head]
		if cur.depth >= maxASBFSDepth {
			continue
		}
		nextDepth := cur.depth + 1
		visit := func(next uint32) bool {
			if next == 0 || visited[next] {
				return false
			}
			visited[next] = true
			parent[next] = cur.asn
			if next == dstASN {
				return true
			}
			queue = append(queue, queueItem{asn: next, depth: nextDepth})
			return false
		}

		// Prefer observed traceroute edges first.
		for _, next := range extraAdj[cur.asn] {
			if visit(next) {
				return nextDepth, reconstructPath(parent, srcASN, dstASN), true
			}
		}
		if g != nil {
			start, end := g.neighborBounds(cur.asn)
			for _, peer := range g.peers[start:end] {
				if visit(peer) {
					return nextDepth, reconstructPath(parent, srcASN, dstASN), true
				}
			}
		}
	}
	return 0, nil, false
}

func reconstructPath(parent map[uint32]uint32, src, dst uint32) []uint32 {
	rev := []uint32{dst}
	cur := dst
	for cur != src {
		p, ok := parent[cur]
		if !ok {
			break
		}
		rev = append(rev, p)
		cur = p
	}
	out := make([]uint32, len(rev))
	for i := range rev {
		out[i] = rev[len(rev)-1-i]
	}
	return out
}
