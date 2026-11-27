// Copyright 2026 Arctel.net
// SPDX-License-Identifier: Apache-2.0

package smartdns

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/model"
)

const (
	jdmNodeInput      = "inputNode"
	jdmNodeOutput     = "outputNode"
	jdmNodeSwitch     = "switchNode"
	jdmNodeExpression = "expressionNode"
)

var validJDMIdentifier = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// PolicyWeights configures relative weights during smart node scoring.
type PolicyWeights struct {
	Latency float64 `json:"latency"`
	Load    float64 `json:"load"`
	Geo     float64 `json:"geo"`
}

// PolicyFilters constrains candidate edge nodes before scoring.
type PolicyFilters struct {
	NodeIDs       []string `json:"node_ids"`
	Countries     []string `json:"countries"`
	Tags          []string `json:"tags"`
	NetworkGroups []string `json:"network_groups"`
}

// PolicyConfig represents structured policy parameters plus an optional visual flow graph.
type PolicyConfig struct {
	Mode          string        `json:"mode"`
	Weights       PolicyWeights `json:"weights"`
	UseTagWeights bool          `json:"use_tag_weights"`
	Filters       PolicyFilters `json:"filters"`
	MaxAnswers    int           `json:"max_answers,omitempty"`
	TTL           int           `json:"ttl,omitempty"`
	Graph         *FlowGraph    `json:"graph,omitempty"`
}

// FlowGraph represents the ReactFlow visual policy builder state.
type FlowGraph struct {
	Nodes []FlowNode `json:"nodes"`
	Edges []FlowEdge `json:"edges"`
}

// FlowNode is one node in the visual policy builder.
type FlowNode struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"` // input | condition | action | output
	Label    string       `json:"label"`
	Position FlowPosition `json:"position"`
	Data     FlowNodeData `json:"data"`
}

// FlowPosition stores UI canvas coordinates.
type FlowPosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// FlowNodeData holds typed configuration for condition/action visual nodes.
type FlowNodeData struct {
	HitPolicy     string           `json:"hit_policy,omitempty"`
	Cases         []FlowSwitchCase `json:"cases,omitempty"`
	Mode          string           `json:"mode,omitempty"`
	Weights       *PolicyWeights   `json:"weights,omitempty"`
	UseTagWeights *bool            `json:"use_tag_weights,omitempty"`
	Filters       *PolicyFilters   `json:"filters,omitempty"`
	MaxAnswers    *int             `json:"max_answers,omitempty"`
	TTL           *int             `json:"ttl,omitempty"`
}

// FlowSwitchCase describes one conditional branch in a visual condition node.
type FlowSwitchCase struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Condition string `json:"condition"`
}

// FlowEdge connects visual builder nodes (and condition case handles).
type FlowEdge struct {
	ID           string `json:"id"`
	Source       string `json:"source"`
	SourceHandle string `json:"source_handle,omitempty"`
	Target       string `json:"target"`
}

// JDMDocument follows the zen-engine 2.x BaseDecisionContent JSON wire format.
type JDMDocument struct {
	Nodes []JDMNode `json:"nodes"`
	Edges []JDMEdge `json:"edges"`
}

// JDMNode is one node inside zen-engine JDM JSON.
type JDMNode struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Type     string         `json:"type"`
	Content  any            `json:"content,omitempty"`
	Position map[string]any `json:"position,omitempty"`
}

// JDMEdge connects two JDM nodes in zen-engine JDM JSON.
type JDMEdge struct {
	ID           string `json:"id"`
	SourceID     string `json:"sourceId"`
	SourceHandle string `json:"sourceHandle,omitempty"`
	TargetID     string `json:"targetId"`
}

// JDMSwitchContent mirrors zen-engine SwitchNodeContent.
type JDMSwitchContent struct {
	HitPolicy  string         `json:"hitPolicy"`
	Statements []JDMStatement `json:"statements"`
}

// JDMStatement mirrors zen-engine SwitchStatement.
type JDMStatement struct {
	ID        string `json:"id"`
	Condition string `json:"condition"`
}

// JDMExpressionContent mirrors zen-engine ExpressionNodeContent ("key" + "value").
type JDMExpressionContent struct {
	Expressions []JDMExpressionItem `json:"expressions"`
}

// JDMExpressionItem represents a single key assignment in an expressionNode.
type JDMExpressionItem struct {
	ID    string `json:"id"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// EvalContext provides client & DNS request fields into JDM condition evaluation.
type EvalContext struct {
	Country  string `json:"country"`
	Region   string `json:"region"`
	ASN      uint32 `json:"asn"`
	IPFamily string `json:"ip_family"`
	Domain   string `json:"domain"`
	QType    string `json:"qtype"`
}

// PolicyDecision is the resolved output produced by JDM evaluation before 4-stage scoring.
type PolicyDecision struct {
	Mode              string        `json:"mode"`
	Weights           PolicyWeights `json:"weights"`
	UseTagWeights     bool          `json:"use_tag_weights"`
	Filters           PolicyFilters `json:"filters"`
	MaxAnswers        int           `json:"max_answers"`
	TTL               int           `json:"ttl"`
	MatchedNodes      []string      `json:"matched_nodes"`
	MatchedConditions []string      `json:"matched_conditions"`
}

func defaultPolicyConfig(strategy string) PolicyConfig {
	mode := model.DNSPolicyStrategySmart
	weights := PolicyWeights{Latency: 0.55, Load: 0.30, Geo: 0.15}
	if strings.EqualFold(strings.TrimSpace(strategy), model.DNSPolicyStrategyLoadBalance) {
		mode = model.DNSPolicyStrategyLoadBalance
		weights = PolicyWeights{Latency: 0, Load: 1, Geo: 0}
	}
	return PolicyConfig{
		Mode:          mode,
		Weights:       weights,
		UseTagWeights: true,
		Filters: PolicyFilters{
			NodeIDs:       []string{},
			Countries:     []string{},
			Tags:          []string{},
			NetworkGroups: []string{},
		},
	}
}

// CompilePresetJDM compiles a preset (smart / load_balance / custom default) PolicyConfig into valid zen-engine JDM JSON.
func CompilePresetJDM(strategy string, cfg PolicyConfig) (string, error) {
	if cfg.Graph != nil && len(cfg.Graph.Nodes) > 0 {
		return CompileFlowGraphToJDM(*cfg.Graph)
	}
	mode := strings.TrimSpace(cfg.Mode)
	if mode == "" {
		if strategy == model.DNSPolicyStrategyLoadBalance {
			mode = model.DNSPolicyStrategyLoadBalance
		} else {
			mode = model.DNSPolicyStrategySmart
		}
	}
	if mode != model.DNSPolicyStrategySmart && mode != model.DNSPolicyStrategyLoadBalance {
		mode = model.DNSPolicyStrategySmart
	}
	exps := buildExpressionsFromAction(FlowNodeData{
		Mode:          mode,
		Weights:       &cfg.Weights,
		UseTagWeights: &cfg.UseTagWeights,
		Filters:       &cfg.Filters,
		MaxAnswers:    intPtrIfPositive(cfg.MaxAnswers),
		TTL:           intPtrIfPositive(cfg.TTL),
	})
	doc := JDMDocument{
		Nodes: []JDMNode{
			{ID: "input", Name: "Request", Type: jdmNodeInput},
			{
				ID:      "expr-default",
				Name:    "Preset Policy",
				Type:    jdmNodeExpression,
				Content: JDMExpressionContent{Expressions: exps},
			},
			{ID: "output", Name: "Decision", Type: jdmNodeOutput},
		},
		Edges: []JDMEdge{
			{ID: "e-in", SourceID: "input", TargetID: "expr-default"},
			{ID: "e-out", SourceID: "expr-default", TargetID: "output"},
		},
	}
	if err := validateJDMDocument(&doc); err != nil {
		return "", err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// CompileFlowGraphToJDM converts the frontend ReactFlow visual graph into validated zen-engine JDM JSON.
func CompileFlowGraphToJDM(graph FlowGraph) (string, error) {
	if len(graph.Nodes) < 3 {
		return "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	doc := JDMDocument{
		Nodes: make([]JDMNode, 0, len(graph.Nodes)),
		Edges: make([]JDMEdge, 0, len(graph.Edges)),
	}
	for _, n := range graph.Nodes {
		id := strings.TrimSpace(n.ID)
		if id == "" || !validJDMIdentifier.MatchString(id) {
			return "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		name := strings.TrimSpace(n.Label)
		if name == "" {
			name = id
		}
		jNode := JDMNode{
			ID:       id,
			Name:     name,
			Position: map[string]any{"x": n.Position.X, "y": n.Position.Y},
		}
		switch strings.ToLower(strings.TrimSpace(n.Type)) {
		case "input", "inputnode":
			jNode.Type = jdmNodeInput
		case "output", "outputnode":
			jNode.Type = jdmNodeOutput
		case "condition", "switch", "switchnode":
			jNode.Type = jdmNodeSwitch
			hitPolicy := strings.ToLower(strings.TrimSpace(n.Data.HitPolicy))
			if hitPolicy == "" {
				hitPolicy = "first"
			}
			if hitPolicy != "first" && hitPolicy != "collect" {
				return "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
			}
			if len(n.Data.Cases) == 0 {
				return "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
			}
			stmts := make([]JDMStatement, 0, len(n.Data.Cases))
			for _, c := range n.Data.Cases {
				cid := strings.TrimSpace(c.ID)
				if cid == "" || !validJDMIdentifier.MatchString(cid) {
					return "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
				}
				stmts = append(stmts, JDMStatement{
					ID:        cid,
					Condition: strings.TrimSpace(c.Condition),
				})
			}
			jNode.Content = JDMSwitchContent{HitPolicy: hitPolicy, Statements: stmts}
		case "action", "expression", "expressionnode":
			jNode.Type = jdmNodeExpression
			exps := buildExpressionsFromAction(n.Data)
			jNode.Content = JDMExpressionContent{Expressions: exps}
		default:
			return "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		doc.Nodes = append(doc.Nodes, jNode)
	}
	for i, e := range graph.Edges {
		edgeID := strings.TrimSpace(e.ID)
		if edgeID == "" {
			edgeID = fmt.Sprintf("edge-%d", i+1)
		}
		doc.Edges = append(doc.Edges, JDMEdge{
			ID:           edgeID,
			SourceID:     strings.TrimSpace(e.Source),
			SourceHandle: strings.TrimSpace(e.SourceHandle),
			TargetID:     strings.TrimSpace(e.Target),
		})
	}
	if err := validateJDMDocument(&doc); err != nil {
		return "", err
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

// ValidateJDMJSON parses and validates a JDM JSON string (structural, acyclic, valid switch/expression payloads).
func ValidateJDMJSON(jdmJSON string) (*JDMDocument, error) {
	var doc JDMDocument
	if err := json.Unmarshal([]byte(jdmJSON), &doc); err != nil {
		return nil, newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	if err := validateJDMDocument(&doc); err != nil {
		return nil, err
	}
	return &doc, nil
}

func validateJDMDocument(doc *JDMDocument) error {
	if doc == nil || len(doc.Nodes) < 2 {
		return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	byID := make(map[string]JDMNode, len(doc.Nodes))
	inputID := ""
	outputID := ""
	for _, n := range doc.Nodes {
		if n.ID == "" {
			return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		if _, dup := byID[n.ID]; dup {
			return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		byID[n.ID] = n
		switch n.Type {
		case jdmNodeInput:
			if inputID != "" {
				return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
			}
			inputID = n.ID
		case jdmNodeOutput:
			if outputID != "" {
				return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
			}
			outputID = n.ID
		case jdmNodeSwitch:
			sw, err := parseSwitchContent(n.Content)
			if err != nil || len(sw.Statements) == 0 {
				return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
			}
			for _, st := range sw.Statements {
				if st.ID == "" {
					return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
				}
				if strings.TrimSpace(st.Condition) != "" {
					if _, err := parseConditionTokens(st.Condition); err != nil {
						return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
					}
				}
			}
		case jdmNodeExpression:
			ex, err := parseExpressionContent(n.Content)
			if err != nil {
				return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
			}
			for _, item := range ex.Expressions {
				if strings.TrimSpace(item.Key) == "" {
					return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
				}
			}
		default:
			return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
	}
	if inputID == "" || outputID == "" {
		return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}

	adj := make(map[string][]string, len(doc.Nodes))
	for _, e := range doc.Edges {
		src, okSrc := byID[e.SourceID]
		_, okDst := byID[e.TargetID]
		if !okSrc || !okDst || e.SourceID == e.TargetID {
			return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		if src.Type == jdmNodeOutput || e.TargetID == inputID {
			return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		adj[e.SourceID] = append(adj[e.SourceID], e.TargetID)
	}

	// Cycle detection (DFS 3-color) & reachability of outputNode from inputNode.
	state := make(map[string]int, len(doc.Nodes))
	reachedOutput := false
	var dfs func(string) bool
	dfs = func(u string) bool {
		state[u] = 1
		if u == outputID {
			reachedOutput = true
		}
		for _, v := range adj[u] {
			if state[v] == 1 {
				return false // cycle detected
			}
			if state[v] == 0 {
				if !dfs(v) {
					return false
				}
			}
		}
		state[u] = 2
		return true
	}
	if !dfs(inputID) || !reachedOutput {
		return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	return nil
}

// EvaluateJDM evaluates a compiled JDM document against an EvalContext, returning resolved policy parameters.
func EvaluateJDM(jdmJSON string, base PolicyDecision, ctx EvalContext) (PolicyDecision, error) {
	doc, err := ValidateJDMJSON(jdmJSON)
	if err != nil {
		return base, err
	}
	byID := make(map[string]JDMNode, len(doc.Nodes))
	outEdges := make(map[string][]JDMEdge, len(doc.Nodes))
	var inputID string
	for _, n := range doc.Nodes {
		byID[n.ID] = n
		if n.Type == jdmNodeInput {
			inputID = n.ID
		}
	}
	for _, e := range doc.Edges {
		outEdges[e.SourceID] = append(outEdges[e.SourceID], e)
	}

	res := base
	queue := []string{inputID}
	visitedOrder := make([]string, 0, len(doc.Nodes))

	for len(queue) > 0 {
		currID := queue[0]
		queue = queue[1:]
		node, ok := byID[currID]
		if !ok {
			continue
		}
		visitedOrder = append(visitedOrder, node.ID)

		switch node.Type {
		case jdmNodeInput:
			for _, e := range outEdges[node.ID] {
				queue = append(queue, e.TargetID)
			}
		case jdmNodeOutput:
			res.MatchedNodes = visitedOrder
			return res, nil
		case jdmNodeExpression:
			ex, err := parseExpressionContent(node.Content)
			if err == nil {
				applyExpressionItems(&res, ex.Expressions)
			}
			for _, e := range outEdges[node.ID] {
				queue = append(queue, e.TargetID)
			}
		case jdmNodeSwitch:
			sw, err := parseSwitchContent(node.Content)
			if err != nil {
				return res, err
			}
			matchedHandles := make(map[string]bool)
			for _, st := range sw.Statements {
				matched, evalErr := evalCondition(st.Condition, ctx)
				if evalErr != nil {
					return res, evalErr
				}
				if matched {
					matchedHandles[st.ID] = true
					condLabel := st.Condition
					if strings.TrimSpace(condLabel) == "" {
						condLabel = "default"
					}
					res.MatchedConditions = append(res.MatchedConditions, fmt.Sprintf("%s:%s", node.ID, condLabel))
					if sw.HitPolicy == "first" {
						break
					}
				}
			}
			for _, e := range outEdges[node.ID] {
				if e.SourceHandle == "" || matchedHandles[e.SourceHandle] {
					queue = append(queue, e.TargetID)
				}
			}
		}
	}
	res.MatchedNodes = visitedOrder
	return res, nil
}

func buildExpressionsFromAction(data FlowNodeData) []JDMExpressionItem {
	exps := make([]JDMExpressionItem, 0, 10)
	mode := strings.TrimSpace(data.Mode)
	if mode == "" {
		mode = model.DNSPolicyStrategySmart
	}
	exps = append(exps, JDMExpressionItem{ID: "mode", Key: "mode", Value: strconv.Quote(mode)})

	if data.Weights != nil {
		exps = append(exps,
			JDMExpressionItem{ID: "w_lat", Key: "weights.latency", Value: formatFloatExpr(data.Weights.Latency)},
			JDMExpressionItem{ID: "w_load", Key: "weights.load", Value: formatFloatExpr(data.Weights.Load)},
			JDMExpressionItem{ID: "w_geo", Key: "weights.geo", Value: formatFloatExpr(data.Weights.Geo)},
		)
	}
	if data.UseTagWeights != nil {
		exps = append(exps, JDMExpressionItem{ID: "use_tags", Key: "use_tag_weights", Value: strconv.FormatBool(*data.UseTagWeights)})
	}
	if data.Filters != nil {
		if len(data.Filters.NodeIDs) > 0 {
			exps = append(exps, JDMExpressionItem{ID: "f_nodes", Key: "filters.node_ids", Value: marshalStringSliceExpr(data.Filters.NodeIDs)})
		}
		if len(data.Filters.Countries) > 0 {
			exps = append(exps, JDMExpressionItem{ID: "f_countries", Key: "filters.countries", Value: marshalStringSliceExpr(data.Filters.Countries)})
		}
		if len(data.Filters.Tags) > 0 {
			exps = append(exps, JDMExpressionItem{ID: "f_tags", Key: "filters.tags", Value: marshalStringSliceExpr(data.Filters.Tags)})
		}
		if len(data.Filters.NetworkGroups) > 0 {
			exps = append(exps, JDMExpressionItem{ID: "f_groups", Key: "filters.network_groups", Value: marshalStringSliceExpr(data.Filters.NetworkGroups)})
		}
	}
	if data.MaxAnswers != nil && *data.MaxAnswers > 0 {
		exps = append(exps, JDMExpressionItem{ID: "max_answers", Key: "max_answers", Value: strconv.Itoa(*data.MaxAnswers)})
	}
	if data.TTL != nil && *data.TTL > 0 {
		exps = append(exps, JDMExpressionItem{ID: "ttl", Key: "ttl", Value: strconv.Itoa(*data.TTL)})
	}
	return exps
}

func formatFloatExpr(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func marshalStringSliceExpr(items []string) string {
	cleaned := make([]string, 0, len(items))
	for _, s := range items {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	raw, _ := json.Marshal(cleaned)
	return string(raw)
}

func intPtrIfPositive(v int) *int {
	if v <= 0 {
		return nil
	}
	cp := v
	return &cp
}

func parseSwitchContent(raw any) (*JDMSwitchContent, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var out JDMSwitchContent
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	if out.HitPolicy == "" {
		out.HitPolicy = "first"
	}
	return &out, nil
}

func parseExpressionContent(raw any) (*JDMExpressionContent, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var out JDMExpressionContent
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func applyExpressionItems(res *PolicyDecision, items []JDMExpressionItem) {
	for _, item := range items {
		val := strings.TrimSpace(item.Value)
		switch strings.TrimSpace(item.Key) {
		case "mode":
			if s, ok := parseJSONOrQuotedString(val); ok && (s == model.DNSPolicyStrategySmart || s == model.DNSPolicyStrategyLoadBalance) {
				res.Mode = s
			}
		case "weights.latency":
			if f, err := strconv.ParseFloat(val, 64); err == nil && f >= 0 {
				res.Weights.Latency = f
			}
		case "weights.load":
			if f, err := strconv.ParseFloat(val, 64); err == nil && f >= 0 {
				res.Weights.Load = f
			}
		case "weights.geo":
			if f, err := strconv.ParseFloat(val, 64); err == nil && f >= 0 {
				res.Weights.Geo = f
			}
		case "use_tag_weights":
			if b, err := strconv.ParseBool(val); err == nil {
				res.UseTagWeights = b
			}
		case "filters.node_ids":
			if slice, ok := parseStringSliceLiteral(val); ok {
				res.Filters.NodeIDs = slice
			}
		case "filters.countries":
			if slice, ok := parseStringSliceLiteral(val); ok {
				res.Filters.Countries = slice
			}
		case "filters.tags":
			if slice, ok := parseStringSliceLiteral(val); ok {
				res.Filters.Tags = slice
			}
		case "filters.network_groups":
			if slice, ok := parseStringSliceLiteral(val); ok {
				res.Filters.NetworkGroups = slice
			}
		case "max_answers":
			if n, err := strconv.Atoi(val); err == nil && n > 0 && n <= 16 {
				res.MaxAnswers = n
			}
		case "ttl":
			if n, err := strconv.Atoi(val); err == nil && n >= 10 && n <= 86400 {
				res.TTL = n
			}
		}
	}
}

func parseJSONOrQuotedString(raw string) (string, bool) {
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		return s, true
	}
	if len(raw) >= 2 && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
		return raw[1 : len(raw)-1], true
	}
	return raw, raw != ""
}

func parseStringSliceLiteral(raw string) ([]string, bool) {
	norm := strings.ReplaceAll(raw, "'", "\"")
	var out []string
	if err := json.Unmarshal([]byte(norm), &out); err != nil {
		return nil, false
	}
	return out, true
}

// Condition tokenizer & recursive-descent evaluator for Zen subset:
// identifiers (client.country, client.region, client.asn, client.ip_family, domain, qtype),
// operators (==, !=, in, contains, &&, ||, !, and, or), literals ("...", '...', numbers, [...]).

type condToken struct {
	kind string // ident, string, number, op, lparen, rparen, lbrack, rbrack, comma
	val  string
}

func parseConditionTokens(expr string) ([]condToken, error) {
	var tokens []condToken
	i := 0
	for i < len(expr) {
		ch := expr[i]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}
		switch ch {
		case '(':
			tokens = append(tokens, condToken{kind: "lparen", val: "("})
			i++
		case ')':
			tokens = append(tokens, condToken{kind: "rparen", val: ")"})
			i++
		case '[':
			tokens = append(tokens, condToken{kind: "lbrack", val: "["})
			i++
		case ']':
			tokens = append(tokens, condToken{kind: "rbrack", val: "]"})
			i++
		case ',':
			tokens = append(tokens, condToken{kind: "comma", val: ","})
			i++
		case '&':
			if i+1 < len(expr) && expr[i+1] == '&' {
				tokens = append(tokens, condToken{kind: "op", val: "&&"})
				i += 2
				continue
			}
			return nil, fmt.Errorf("unexpected char '&'")
		case '|':
			if i+1 < len(expr) && expr[i+1] == '|' {
				tokens = append(tokens, condToken{kind: "op", val: "||"})
				i += 2
				continue
			}
			return nil, fmt.Errorf("unexpected char '|'")
		case '=':
			if i+1 < len(expr) && expr[i+1] == '=' {
				tokens = append(tokens, condToken{kind: "op", val: "=="})
				i += 2
				continue
			}
			return nil, fmt.Errorf("unexpected char '='")
		case '!':
			if i+1 < len(expr) && expr[i+1] == '=' {
				tokens = append(tokens, condToken{kind: "op", val: "!="})
				i += 2
				continue
			}
			tokens = append(tokens, condToken{kind: "op", val: "!"})
			i++
		case '"', '\'':
			quote := ch
			i++
			start := i
			for i < len(expr) && expr[i] != quote {
				if expr[i] == '\\' && i+1 < len(expr) {
					i++
				}
				i++
			}
			if i >= len(expr) {
				return nil, fmt.Errorf("unterminated string")
			}
			tokens = append(tokens, condToken{kind: "string", val: expr[start:i]})
			i++
		default:
			if ch >= '0' && ch <= '9' {
				start := i
				for i < len(expr) && (expr[i] >= '0' && expr[i] <= '9') {
					i++
				}
				tokens = append(tokens, condToken{kind: "number", val: expr[start:i]})
				continue
			}
			if isIdentStart(ch) {
				start := i
				for i < len(expr) && (isIdentPart(expr[i])) {
					i++
				}
				word := expr[start:i]
				switch word {
				case "and":
					tokens = append(tokens, condToken{kind: "op", val: "&&"})
				case "or":
					tokens = append(tokens, condToken{kind: "op", val: "||"})
				case "not":
					tokens = append(tokens, condToken{kind: "op", val: "!"})
				case "in", "contains":
					tokens = append(tokens, condToken{kind: "op", val: word})
				default:
					tokens = append(tokens, condToken{kind: "ident", val: word})
				}
				continue
			}
			return nil, fmt.Errorf("unexpected character %q", ch)
		}
	}
	return tokens, nil
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '.'
}

func evalCondition(expr string, ctx EvalContext) (bool, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" || trimmed == "true" {
		return true, nil
	}
	if trimmed == "false" {
		return false, nil
	}
	tokens, err := parseConditionTokens(trimmed)
	if err != nil {
		return false, err
	}
	p := &condParser{tokens: tokens, ctx: ctx}
	res, err := p.parseOr()
	if err != nil {
		return false, err
	}
	if p.pos != len(p.tokens) {
		return false, fmt.Errorf("unexpected trailing token %q", p.tokens[p.pos].val)
	}
	return res, nil
}

type condParser struct {
	tokens []condToken
	pos    int
	ctx    EvalContext
}

func (p *condParser) peek() (condToken, bool) {
	if p.pos >= len(p.tokens) {
		return condToken{}, false
	}
	return p.tokens[p.pos], true
}

func (p *condParser) next() (condToken, bool) {
	if p.pos >= len(p.tokens) {
		return condToken{}, false
	}
	tok := p.tokens[p.pos]
	p.pos++
	return tok, true
}

func (p *condParser) parseOr() (bool, error) {
	left, err := p.parseAnd()
	if err != nil {
		return false, err
	}
	for {
		tok, ok := p.peek()
		if !ok || tok.kind != "op" || tok.val != "||" {
			break
		}
		p.pos++
		right, err := p.parseAnd()
		if err != nil {
			return false, err
		}
		left = left || right
	}
	return left, nil
}

func (p *condParser) parseAnd() (bool, error) {
	left, err := p.parseUnary()
	if err != nil {
		return false, err
	}
	for {
		tok, ok := p.peek()
		if !ok || tok.kind != "op" || tok.val != "&&" {
			break
		}
		p.pos++
		right, err := p.parseUnary()
		if err != nil {
			return false, err
		}
		left = left && right
	}
	return left, nil
}

func (p *condParser) parseUnary() (bool, error) {
	if tok, ok := p.peek(); ok && tok.kind == "op" && tok.val == "!" {
		p.pos++
		val, err := p.parseUnary()
		return !val, err
	}
	return p.parsePrimary()
}

func (p *condParser) parsePrimary() (bool, error) {
	tok, ok := p.peek()
	if !ok {
		return false, fmt.Errorf("unexpected end of condition")
	}
	if tok.kind == "lparen" {
		p.pos++
		val, err := p.parseOr()
		if err != nil {
			return false, err
		}
		closeTok, ok := p.next()
		if !ok || closeTok.kind != "rparen" {
			return false, fmt.Errorf("missing ')'")
		}
		return val, nil
	}
	if tok.kind == "ident" && tok.val == "contains" {
		p.pos++
		if openTok, ok := p.next(); !ok || openTok.kind != "lparen" {
			return false, fmt.Errorf("expected '(' after contains")
		}
		aTok, ok := p.next()
		if !ok {
			return false, fmt.Errorf("expected arg1")
		}
		if commaTok, ok := p.next(); !ok || commaTok.kind != "comma" {
			return false, fmt.Errorf("expected ',' in contains")
		}
		bTok, ok := p.next()
		if !ok {
			return false, fmt.Errorf("expected arg2")
		}
		if closeTok, ok := p.next(); !ok || closeTok.kind != "rparen" {
			return false, fmt.Errorf("expected ')' after contains")
		}
		leftVal := p.resolveValue(aTok)
		rightVal := p.resolveValue(bTok)
		return strings.Contains(strings.ToLower(leftVal), strings.ToLower(rightVal)), nil
	}
	// Boolean literal or comparison: lhs op rhs
	lhs, ok := p.next()
	if !ok {
		return false, fmt.Errorf("expected operand")
	}
	if lhs.kind == "ident" && (lhs.val == "true" || lhs.val == "false") {
		if op, hasOp := p.peek(); !hasOp || op.kind != "op" {
			return lhs.val == "true", nil
		}
	}
	opTok, ok := p.next()
	if !ok || opTok.kind != "op" {
		return false, fmt.Errorf("expected comparison operator")
	}
	leftStr := p.resolveValue(lhs)
	switch opTok.val {
	case "==", "!=":
		rhs, ok := p.next()
		if !ok {
			return false, fmt.Errorf("expected rhs value")
		}
		rightStr := p.resolveValue(rhs)
		eq := strings.EqualFold(leftStr, rightStr)
		if opTok.val == "!=" {
			return !eq, nil
		}
		return eq, nil
	case "contains":
		rhs, ok := p.next()
		if !ok {
			return false, fmt.Errorf("expected rhs value")
		}
		rightStr := p.resolveValue(rhs)
		return strings.Contains(strings.ToLower(leftStr), strings.ToLower(rightStr)), nil
	case "in":
		listVals, err := p.parseList()
		if err != nil {
			return false, err
		}
		for _, item := range listVals {
			if strings.EqualFold(leftStr, item) {
				return true, nil
			}
		}
		return false, nil
	default:
		return false, fmt.Errorf("unsupported operator %q", opTok.val)
	}
}

func (p *condParser) parseList() ([]string, error) {
	openTok, ok := p.next()
	if !ok || openTok.kind != "lbrack" {
		return nil, fmt.Errorf("expected '[' after in")
	}
	var values []string
	for {
		tok, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated '['")
		}
		if tok.kind == "rbrack" {
			p.pos++
			break
		}
		valTok, _ := p.next()
		values = append(values, p.resolveValue(valTok))
		if nextTok, ok := p.peek(); ok && nextTok.kind == "comma" {
			p.pos++
		}
	}
	return values, nil
}

func (p *condParser) resolveValue(tok condToken) string {
	if tok.kind == "string" || tok.kind == "number" {
		return tok.val
	}
	switch strings.ToLower(tok.val) {
	case "client.country":
		return strings.ToUpper(strings.TrimSpace(p.ctx.Country))
	case "client.region":
		return strings.ToUpper(strings.TrimSpace(p.ctx.Region))
	case "client.asn":
		return strconv.FormatUint(uint64(p.ctx.ASN), 10)
	case "client.ip_family":
		return strings.ToLower(strings.TrimSpace(p.ctx.IPFamily))
	case "domain":
		return strings.ToLower(strings.TrimSpace(p.ctx.Domain))
	case "qtype":
		return strings.ToUpper(strings.TrimSpace(p.ctx.QType))
	default:
		return tok.val
	}
}
