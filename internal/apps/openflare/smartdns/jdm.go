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

	jdmNodeIDInput       = "input"
	jdmNodeIDDefaultExpr = "expr-default"
	jdmNodeIDOutput      = "output"
	jdmHitPolicyFirst    = "first"
	jdmHitPolicyCollect  = "collect"

	jdmKeyMode         = "mode"
	jdmKeyMaxAnswers   = "max_answers"
	jdmKeyTTL          = "ttl"
	jdmKeyUseTags      = "use_tag_weights"
	jdmKeyWeightLat    = "weights.latency"
	jdmKeyWeightLoad   = "weights.load"
	jdmKeyWeightGeo    = "weights.geo"
	jdmKeyFilterNodes  = "filters.node_ids"
	jdmKeyFilterCtry   = "filters.countries"
	jdmKeyFilterTags   = "filters.tags"
	jdmKeyFilterGroups = "filters.network_groups"

	tokKindIdent  = "ident"
	tokKindString = "string"
	tokKindNumber = "number"
	tokKindOp     = "op"
	tokKindLParen = "lparen"
	tokKindRParen = "rparen"
	tokKindLBrack = "lbrack"
	tokKindRBrack = "rbrack"
	tokKindComma  = "comma"

	opContains          = "contains"
	opIn                = "in"
	boolLiteralTrue     = dnsQuarantinedHeaderTrue
	boolLiteralFalse    = "false"
	minJDMDocumentNodes = 2
	minFlowGraphNodes   = 3
	defaultExprCap      = 10
	minQuotedStringLen  = 2
	dfsVisiting         = 1
	dfsVisited          = 2
	twoCharOpLen        = 2
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
			{ID: jdmNodeIDInput, Name: "Request", Type: jdmNodeInput},
			{
				ID:      jdmNodeIDDefaultExpr,
				Name:    "Preset Policy",
				Type:    jdmNodeExpression,
				Content: JDMExpressionContent{Expressions: exps},
			},
			{ID: jdmNodeIDOutput, Name: "Decision", Type: jdmNodeOutput},
		},
		Edges: []JDMEdge{
			{ID: "e-in", SourceID: jdmNodeIDInput, TargetID: jdmNodeIDDefaultExpr},
			{ID: "e-out", SourceID: jdmNodeIDDefaultExpr, TargetID: jdmNodeIDOutput},
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
	if len(graph.Nodes) < minFlowGraphNodes {
		return "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	doc := JDMDocument{
		Nodes: make([]JDMNode, 0, len(graph.Nodes)),
		Edges: make([]JDMEdge, 0, len(graph.Edges)),
	}
	for _, n := range graph.Nodes {
		jNode, err := compileFlowNode(n)
		if err != nil {
			return "", err
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

func compileFlowNode(n FlowNode) (JDMNode, error) {
	id := strings.TrimSpace(n.ID)
	if id == "" || !validJDMIdentifier.MatchString(id) {
		return JDMNode{}, newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
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
	case jdmNodeIDInput, "inputnode":
		jNode.Type = jdmNodeInput
	case jdmNodeIDOutput, "outputnode":
		jNode.Type = jdmNodeOutput
	case "condition", "switch", "switchnode":
		sw, err := compileSwitchFlowContent(n.Data)
		if err != nil {
			return JDMNode{}, err
		}
		jNode.Type = jdmNodeSwitch
		jNode.Content = sw
	case "action", "expression", "expressionnode":
		jNode.Type = jdmNodeExpression
		jNode.Content = JDMExpressionContent{Expressions: buildExpressionsFromAction(n.Data)}
	default:
		return JDMNode{}, newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	return jNode, nil
}

func compileSwitchFlowContent(data FlowNodeData) (JDMSwitchContent, error) {
	hitPolicy := strings.ToLower(strings.TrimSpace(data.HitPolicy))
	if hitPolicy == "" {
		hitPolicy = jdmHitPolicyFirst
	}
	if hitPolicy != jdmHitPolicyFirst && hitPolicy != jdmHitPolicyCollect {
		return JDMSwitchContent{}, newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	if len(data.Cases) == 0 {
		return JDMSwitchContent{}, newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	stmts := make([]JDMStatement, 0, len(data.Cases))
	for _, c := range data.Cases {
		cid := strings.TrimSpace(c.ID)
		if cid == "" || !validJDMIdentifier.MatchString(cid) {
			return JDMSwitchContent{}, newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		stmts = append(stmts, JDMStatement{
			ID:        cid,
			Condition: strings.TrimSpace(c.Condition),
		})
	}
	return JDMSwitchContent{HitPolicy: hitPolicy, Statements: stmts}, nil
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
	if doc == nil || len(doc.Nodes) < minJDMDocumentNodes {
		return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	byID, inputID, outputID, err := validateJDMNodes(doc.Nodes)
	if err != nil {
		return err
	}
	return validateJDMGraphDAG(doc, byID, inputID, outputID)
}

func validateJDMNodes(nodes []JDMNode) (map[string]JDMNode, string, string, error) {
	byID := make(map[string]JDMNode, len(nodes))
	inputID := ""
	outputID := ""
	for _, n := range nodes {
		if n.ID == "" {
			return nil, "", "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		if _, dup := byID[n.ID]; dup {
			return nil, "", "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		byID[n.ID] = n
		var err error
		inputID, outputID, err = validateSingleJDMNode(n, inputID, outputID)
		if err != nil {
			return nil, "", "", err
		}
	}
	if inputID == "" || outputID == "" {
		return nil, "", "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	return byID, inputID, outputID, nil
}

func validateSingleJDMNode(n JDMNode, inputID, outputID string) (string, string, error) {
	switch n.Type {
	case jdmNodeInput:
		if inputID != "" {
			return "", "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		return n.ID, outputID, nil
	case jdmNodeOutput:
		if outputID != "" {
			return "", "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		return inputID, n.ID, nil
	case jdmNodeSwitch:
		return inputID, outputID, validateJDMSwitchNode(n.Content)
	case jdmNodeExpression:
		return inputID, outputID, validateJDMExpressionNode(n.Content)
	default:
		return "", "", newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
}

func validateJDMSwitchNode(content any) error {
	sw, err := parseSwitchContent(content)
	if err != nil || len(sw.Statements) == 0 {
		return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	for _, st := range sw.Statements {
		if st.ID == "" {
			return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
		if strings.TrimSpace(st.Condition) != "" {
			if _, tokErr := parseConditionTokens(st.Condition); tokErr != nil {
				return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
			}
		}
	}
	return nil
}

func validateJDMExpressionNode(content any) error {
	ex, err := parseExpressionContent(content)
	if err != nil {
		return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
	}
	for _, item := range ex.Expressions {
		if strings.TrimSpace(item.Key) == "" {
			return newLogicError(http.StatusBadRequest, errPolicyGraphInvalid)
		}
	}
	return nil
}

func validateJDMGraphDAG(doc *JDMDocument, byID map[string]JDMNode, inputID, outputID string) error {
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
		state[u] = dfsVisiting
		if u == outputID {
			reachedOutput = true
		}
		for _, v := range adj[u] {
			if state[v] == dfsVisiting {
				return false // cycle detected
			}
			if state[v] == 0 && !dfs(v) {
				return false
			}
		}
		state[u] = dfsVisited
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
	byID, outEdges, inputID := indexJDMGraph(doc)

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

		if node.Type == jdmNodeOutput {
			res.MatchedNodes = visitedOrder
			return res, nil
		}
		var stepErr error
		queue, stepErr = stepJDMNode(node, outEdges[node.ID], queue, &res, ctx)
		if stepErr != nil {
			return res, stepErr
		}
	}
	res.MatchedNodes = visitedOrder
	return res, nil
}

func indexJDMGraph(doc *JDMDocument) (map[string]JDMNode, map[string][]JDMEdge, string) {
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
	return byID, outEdges, inputID
}

func stepJDMNode(
	node JDMNode,
	edges []JDMEdge,
	queue []string,
	res *PolicyDecision,
	ctx EvalContext,
) ([]string, error) {
	switch node.Type {
	case jdmNodeInput:
		for _, e := range edges {
			queue = append(queue, e.TargetID)
		}
	case jdmNodeExpression:
		if ex, err := parseExpressionContent(node.Content); err == nil {
			applyExpressionItems(res, ex.Expressions)
		}
		for _, e := range edges {
			queue = append(queue, e.TargetID)
		}
	case jdmNodeSwitch:
		return evalSwitchEdges(node, edges, queue, res, ctx)
	}
	return queue, nil
}

func evalSwitchEdges(
	node JDMNode,
	edges []JDMEdge,
	queue []string,
	res *PolicyDecision,
	ctx EvalContext,
) ([]string, error) {
	sw, err := parseSwitchContent(node.Content)
	if err != nil {
		return queue, err
	}
	matchedHandles := make(map[string]bool)
	for _, st := range sw.Statements {
		matched, evalErr := evalCondition(st.Condition, ctx)
		if evalErr != nil {
			return queue, evalErr
		}
		if !matched {
			continue
		}
		matchedHandles[st.ID] = true
		condLabel := st.Condition
		if strings.TrimSpace(condLabel) == "" {
			condLabel = "default"
		}
		res.MatchedConditions = append(res.MatchedConditions, fmt.Sprintf("%s:%s", node.ID, condLabel))
		if sw.HitPolicy == jdmHitPolicyFirst {
			break
		}
	}
	for _, e := range edges {
		if e.SourceHandle == "" || matchedHandles[e.SourceHandle] {
			queue = append(queue, e.TargetID)
		}
	}
	return queue, nil
}

func buildExpressionsFromAction(data FlowNodeData) []JDMExpressionItem {
	exps := make([]JDMExpressionItem, 0, defaultExprCap)
	mode := strings.TrimSpace(data.Mode)
	if mode == "" {
		mode = model.DNSPolicyStrategySmart
	}
	exps = append(exps, JDMExpressionItem{ID: jdmKeyMode, Key: jdmKeyMode, Value: strconv.Quote(mode)})

	if data.Weights != nil {
		exps = append(exps,
			JDMExpressionItem{ID: "w_lat", Key: jdmKeyWeightLat, Value: formatFloatExpr(data.Weights.Latency)},
			JDMExpressionItem{ID: "w_load", Key: jdmKeyWeightLoad, Value: formatFloatExpr(data.Weights.Load)},
			JDMExpressionItem{ID: "w_geo", Key: jdmKeyWeightGeo, Value: formatFloatExpr(data.Weights.Geo)},
		)
	}
	if data.UseTagWeights != nil {
		exps = append(exps, JDMExpressionItem{ID: "use_tags", Key: jdmKeyUseTags, Value: strconv.FormatBool(*data.UseTagWeights)})
	}
	if data.Filters != nil {
		if len(data.Filters.NodeIDs) > 0 {
			exps = append(exps, JDMExpressionItem{ID: "f_nodes", Key: jdmKeyFilterNodes, Value: marshalStringSliceExpr(data.Filters.NodeIDs)})
		}
		if len(data.Filters.Countries) > 0 {
			exps = append(exps, JDMExpressionItem{ID: "f_countries", Key: jdmKeyFilterCtry, Value: marshalStringSliceExpr(data.Filters.Countries)})
		}
		if len(data.Filters.Tags) > 0 {
			exps = append(exps, JDMExpressionItem{ID: "f_tags", Key: jdmKeyFilterTags, Value: marshalStringSliceExpr(data.Filters.Tags)})
		}
		if len(data.Filters.NetworkGroups) > 0 {
			exps = append(exps, JDMExpressionItem{ID: "f_groups", Key: jdmKeyFilterGroups, Value: marshalStringSliceExpr(data.Filters.NetworkGroups)})
		}
	}
	if data.MaxAnswers != nil && *data.MaxAnswers > 0 {
		exps = append(exps, JDMExpressionItem{ID: jdmKeyMaxAnswers, Key: jdmKeyMaxAnswers, Value: strconv.Itoa(*data.MaxAnswers)})
	}
	if data.TTL != nil && *data.TTL > 0 {
		exps = append(exps, JDMExpressionItem{ID: jdmKeyTTL, Key: jdmKeyTTL, Value: strconv.Itoa(*data.TTL)})
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
		out.HitPolicy = jdmHitPolicyFirst
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
		applySingleExpression(res, strings.TrimSpace(item.Key), strings.TrimSpace(item.Value))
	}
}

func applySingleExpression(res *PolicyDecision, key, val string) {
	if applyWeightExpression(res, key, val) || applyFilterExpression(res, key, val) {
		return
	}
	switch key {
	case jdmKeyMode:
		if s, ok := parseJSONOrQuotedString(val); ok && (s == model.DNSPolicyStrategySmart || s == model.DNSPolicyStrategyLoadBalance) {
			res.Mode = s
		}
	case jdmKeyUseTags:
		if b, err := strconv.ParseBool(val); err == nil {
			res.UseTagWeights = b
		}
	case jdmKeyMaxAnswers:
		if n, err := strconv.Atoi(val); err == nil && n >= minPolicyMaxAnswers && n <= maxPolicyMaxAnswers {
			res.MaxAnswers = n
		}
	case jdmKeyTTL:
		if n, err := strconv.Atoi(val); err == nil && n >= minPolicyTTL && n <= maxPolicyTTL {
			res.TTL = n
		}
	}
}

func applyWeightExpression(res *PolicyDecision, key, val string) bool {
	var target *float64
	switch key {
	case jdmKeyWeightLat:
		target = &res.Weights.Latency
	case jdmKeyWeightLoad:
		target = &res.Weights.Load
	case jdmKeyWeightGeo:
		target = &res.Weights.Geo
	default:
		return false
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil && f >= 0 {
		*target = f
	}
	return true
}

func applyFilterExpression(res *PolicyDecision, key, val string) bool {
	var target *[]string
	switch key {
	case jdmKeyFilterNodes:
		target = &res.Filters.NodeIDs
	case jdmKeyFilterCtry:
		target = &res.Filters.Countries
	case jdmKeyFilterTags:
		target = &res.Filters.Tags
	case jdmKeyFilterGroups:
		target = &res.Filters.NetworkGroups
	default:
		return false
	}
	if slice, ok := parseStringSliceLiteral(val); ok {
		*target = slice
	}
	return true
}

func parseJSONOrQuotedString(raw string) (string, bool) {
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		return s, true
	}
	if len(raw) >= minQuotedStringLen && raw[0] == '\'' && raw[len(raw)-1] == '\'' {
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
		tok, nextIdx, err := nextConditionToken(expr, i)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
		i = nextIdx
	}
	return tokens, nil
}

func nextConditionToken(expr string, i int) (condToken, int, error) {
	ch := expr[i]
	if tok, ok := delimiterToken(ch); ok {
		return tok, i + 1, nil
	}
	if ch == '"' || ch == '\'' {
		return scanQuotedConditionToken(expr, i, ch)
	}
	if tok, nextIdx, handled, err := scanOperatorToken(expr, i, ch); handled || err != nil {
		return tok, nextIdx, err
	}
	if ch >= '0' && ch <= '9' {
		end := i
		for end < len(expr) && expr[end] >= '0' && expr[end] <= '9' {
			end++
		}
		return condToken{kind: tokKindNumber, val: expr[i:end]}, end, nil
	}
	if isIdentStart(ch) {
		return scanWordConditionToken(expr, i), scanWordEnd(expr, i), nil
	}
	return condToken{}, i, fmt.Errorf("unexpected character %q", ch)
}

func delimiterToken(ch byte) (condToken, bool) {
	switch ch {
	case '(':
		return condToken{kind: tokKindLParen, val: "("}, true
	case ')':
		return condToken{kind: tokKindRParen, val: ")"}, true
	case '[':
		return condToken{kind: tokKindLBrack, val: "["}, true
	case ']':
		return condToken{kind: tokKindRBrack, val: "]"}, true
	case ',':
		return condToken{kind: tokKindComma, val: ","}, true
	default:
		return condToken{}, false
	}
}

func scanOperatorToken(expr string, i int, ch byte) (condToken, int, bool, error) {
	nextCh := byte(0)
	if i+1 < len(expr) {
		nextCh = expr[i+1]
	}
	switch ch {
	case '&':
		if nextCh == '&' {
			return condToken{kind: tokKindOp, val: "&&"}, i + twoCharOpLen, true, nil
		}
		return condToken{}, i, true, fmt.Errorf("unexpected char '&'")
	case '|':
		if nextCh == '|' {
			return condToken{kind: tokKindOp, val: "||"}, i + twoCharOpLen, true, nil
		}
		return condToken{}, i, true, fmt.Errorf("unexpected char '|'")
	case '=':
		if nextCh == '=' {
			return condToken{kind: tokKindOp, val: "=="}, i + twoCharOpLen, true, nil
		}
		return condToken{}, i, true, fmt.Errorf("unexpected char '='")
	case '!':
		if nextCh == '=' {
			return condToken{kind: tokKindOp, val: "!="}, i + twoCharOpLen, true, nil
		}
		return condToken{kind: tokKindOp, val: "!"}, i + 1, true, nil
	default:
		return condToken{}, i, false, nil
	}
}

func scanQuotedConditionToken(expr string, i int, quote byte) (condToken, int, error) {
	pos := i + 1
	start := pos
	for pos < len(expr) && expr[pos] != quote {
		if expr[pos] == '\\' && pos+1 < len(expr) {
			pos++
		}
		pos++
	}
	if pos >= len(expr) {
		return condToken{}, pos, fmt.Errorf("unterminated string")
	}
	return condToken{kind: tokKindString, val: expr[start:pos]}, pos + 1, nil
}

func scanWordEnd(expr string, i int) int {
	pos := i
	for pos < len(expr) && isIdentPart(expr[pos]) {
		pos++
	}
	return pos
}

func scanWordConditionToken(expr string, i int) condToken {
	word := expr[i:scanWordEnd(expr, i)]
	switch word {
	case "and":
		return condToken{kind: tokKindOp, val: "&&"}
	case "or":
		return condToken{kind: tokKindOp, val: "||"}
	case "not":
		return condToken{kind: tokKindOp, val: "!"}
	case opIn, opContains:
		return condToken{kind: tokKindOp, val: word}
	default:
		return condToken{kind: tokKindIdent, val: word}
	}
}

func isIdentStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9') || c == '.'
}

func evalCondition(expr string, ctx EvalContext) (bool, error) {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" || trimmed == boolLiteralTrue {
		return true, nil
	}
	if trimmed == boolLiteralFalse {
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
		if !ok || tok.kind != tokKindOp || tok.val != "||" {
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
		if !ok || tok.kind != tokKindOp || tok.val != "&&" {
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
	if tok, ok := p.peek(); ok && tok.kind == tokKindOp && tok.val == "!" {
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
	if tok.kind == tokKindLParen {
		return p.parseGroupedExpr()
	}
	if tok.kind == tokKindIdent && tok.val == opContains {
		return p.parseContainsCall()
	}
	return p.parseComparisonExpr()
}

func (p *condParser) parseGroupedExpr() (bool, error) {
	p.pos++
	val, err := p.parseOr()
	if err != nil {
		return false, err
	}
	closeTok, ok := p.next()
	if !ok || closeTok.kind != tokKindRParen {
		return false, fmt.Errorf("missing ')'")
	}
	return val, nil
}

func (p *condParser) parseContainsCall() (bool, error) {
	p.pos++
	if openTok, ok := p.next(); !ok || openTok.kind != tokKindLParen {
		return false, fmt.Errorf("expected '(' after contains")
	}
	aTok, ok := p.next()
	if !ok {
		return false, fmt.Errorf("expected arg1")
	}
	if commaTok, ok := p.next(); !ok || commaTok.kind != tokKindComma {
		return false, fmt.Errorf("expected ',' in contains")
	}
	bTok, ok := p.next()
	if !ok {
		return false, fmt.Errorf("expected arg2")
	}
	if closeTok, ok := p.next(); !ok || closeTok.kind != tokKindRParen {
		return false, fmt.Errorf("expected ')' after contains")
	}
	return strings.Contains(strings.ToLower(p.resolveValue(aTok)), strings.ToLower(p.resolveValue(bTok))), nil
}

func (p *condParser) parseComparisonExpr() (bool, error) {
	lhs, ok := p.next()
	if !ok {
		return false, fmt.Errorf("expected operand")
	}
	if lhs.kind == tokKindIdent && (lhs.val == boolLiteralTrue || lhs.val == boolLiteralFalse) {
		if op, hasOp := p.peek(); !hasOp || op.kind != tokKindOp {
			return lhs.val == boolLiteralTrue, nil
		}
	}
	opTok, ok := p.next()
	if !ok || opTok.kind != tokKindOp {
		return false, fmt.Errorf("expected comparison operator")
	}
	leftStr := p.resolveValue(lhs)
	switch opTok.val {
	case "==", "!=":
		return p.evalEqualityOp(leftStr, opTok.val)
	case opContains:
		rhs, ok := p.next()
		if !ok {
			return false, fmt.Errorf("expected rhs value")
		}
		return strings.Contains(strings.ToLower(leftStr), strings.ToLower(p.resolveValue(rhs))), nil
	case opIn:
		return p.evalInListOp(leftStr)
	default:
		return false, fmt.Errorf("unsupported operator %q", opTok.val)
	}
}

func (p *condParser) evalEqualityOp(leftStr, op string) (bool, error) {
	rhs, ok := p.next()
	if !ok {
		return false, fmt.Errorf("expected rhs value")
	}
	eq := strings.EqualFold(leftStr, p.resolveValue(rhs))
	if op == "!=" {
		return !eq, nil
	}
	return eq, nil
}

func (p *condParser) evalInListOp(leftStr string) (bool, error) {
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
}

func (p *condParser) parseList() ([]string, error) {
	openTok, ok := p.next()
	if !ok || openTok.kind != tokKindLBrack {
		return nil, fmt.Errorf("expected '[' after in")
	}
	var values []string
	for {
		tok, ok := p.peek()
		if !ok {
			return nil, fmt.Errorf("unterminated '['")
		}
		if tok.kind == tokKindRBrack {
			p.pos++
			break
		}
		valTok, _ := p.next()
		values = append(values, p.resolveValue(valTok))
		if nextTok, ok := p.peek(); ok && nextTok.kind == tokKindComma {
			p.pos++
		}
	}
	return values, nil
}

func (p *condParser) resolveValue(tok condToken) string {
	if tok.kind == tokKindString || tok.kind == tokKindNumber {
		return tok.val
	}
	switch strings.ToLower(tok.val) {
	case "client.country":
		return strings.ToUpper(strings.TrimSpace(p.ctx.Country))
	case "client.region":
		return strings.ToUpper(strings.TrimSpace(p.ctx.Region))
	case "client.asn":
		return strconv.FormatUint(uint64(p.ctx.ASN), decimalBase)
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
