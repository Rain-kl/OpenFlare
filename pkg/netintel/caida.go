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
	caidaMinEdgeFields    = 3
	caidaBFSInitialCap    = 64
)

type caidaGraph struct {
	neighbors map[uint32][]ASRelation
}

func loadCAIDAGraph(dir string, manifest *Manifest) (*caidaGraph, DatasetFileStatus) {
	graph := &caidaGraph{
		neighbors: make(map[uint32][]ASRelation),
	}
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
	edges := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < caidaMinEdgeFields {
			continue
		}
		as1, ok1 := parseASNNumber(parts[0])
		as2, ok2 := parseASNNumber(parts[1])
		rel, err := strconv.Atoi(strings.TrimSpace(parts[2]))
		if !ok1 || !ok2 || err != nil || as1 == 0 || as2 == 0 || as1 == as2 {
			continue
		}
		switch rel {
		case -1:
			graph.neighbors[as1] = append(graph.neighbors[as1], ASRelation{PeerASN: as2, Relation: RelationCustomer})
			graph.neighbors[as2] = append(graph.neighbors[as2], ASRelation{PeerASN: as1, Relation: RelationProvider})
			edges++
		case 0:
			graph.neighbors[as1] = append(graph.neighbors[as1], ASRelation{PeerASN: as2, Relation: RelationPeer})
			graph.neighbors[as2] = append(graph.neighbors[as2], ASRelation{PeerASN: as1, Relation: RelationPeer})
			edges++
		}
	}
	return edges, scanner.Err()
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
			for _, rel := range g.neighbors[cur.asn] {
				if visit(rel.PeerASN) {
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
