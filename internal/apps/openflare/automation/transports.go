package automation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/Rain-kl/Wavelet/internal/automationcontract"
	"github.com/Rain-kl/Wavelet/internal/platform/buildinfo"
	"github.com/Rain-kl/Wavelet/internal/shared/response"
	"github.com/Rain-kl/Wavelet/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	requestIDHeader  = "X-OpenFlare-Request-Id"
	errorCodeHeader  = "X-OpenFlare-Error-Code"
	maxOperationBody = 128 << 10
)

type actorKey struct{}

// OperationResponse is the REST response envelope documented in Swagger.
type OperationResponse = response.Any

func authorizeTransport(c *gin.Context, site instance, isMCP bool) (identity, *operationError) {
	resource := site.apiResource()
	if isMCP {
		resource = site.mcpResource()
	}
	actor, err := authenticate(c.Request, site, resource)
	if err == nil {
		return actor, nil
	}
	if !isInvalidCredential(err) {
		return identity{}, operationFailed(c.Request.Context(), "authenticate", err)
	}
	metadataURL := site.origin + metadataPath + strings.TrimPrefix(resource, site.origin)
	c.Header("WWW-Authenticate", fmt.Sprintf(`Bearer resource_metadata=%q, scope=%q`, metadataURL, strings.Join(availableScopes(), " ")))
	return identity{}, &operationError{http.StatusUnauthorized, "UNAUTHORIZED", "Sign in using an automation client for this instance."}
}

// RegisterAPI mounts the authenticated automation catalog and REST operations.
func RegisterAPI(group *gin.RouterGroup) {
	operations := bindings()
	group.Use(requireAPIAuthorization)
	group.GET("/operations", listOperations(operations))
	for _, operation := range operations {
		path := routeParameters.ReplaceAllString(operation.Path, ":${1}")
		group.Handle(operation.Method, path, serveOperation(operation))
	}
}

var routeParameters = regexp.MustCompile(`\{([^{}]+)\}`)

func requireAPIAuthorization(c *gin.Context) {
	c.Header(requestIDHeader, requestID())
	c.Header("Cache-Control", "no-store")
	site, err := loadInstance(c.Request.Context())
	if errors.Is(err, errDisabled) {
		apiError(c, &operationError{http.StatusNotFound, "AUTOMATION_DISABLED", "Automation is disabled on this instance."})
		return
	}
	if err != nil {
		apiError(c, operationFailed(c.Request.Context(), "load configuration", err))
		return
	}
	actor, failure := authorizeTransport(c, site, false)
	if failure != nil {
		apiError(c, failure)
		return
	}
	c.Header(automationcontract.InstanceHeader, actor.instanceID)
	c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), actorKey{}, actor))
	c.Next()
}

// @Summary List authorized automation operations
// @Description Return the connected instance ID and the published contracts available to the current credential.
// @Tags automation
// @Produce json
// @Security AutomationBearer
// @Success 200 {object} response.Any{data=Catalog}
// @Failure 401 {object} response.Any
// @Failure 404 {object} response.Any
// @Failure 500 {object} response.Any
// @Router /api/v1/automation/operations [get]
func listOperations(operations []binding) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, _ := c.Request.Context().Value(actorKey{}).(identity)
		result := Catalog{InstanceID: actor.instanceID, Operations: make([]automationcontract.Operation, 0)}
		for _, operation := range operations {
			if operation.permitted(actor) {
				result.Operations = append(result.Operations, operation.Operation)
			}
		}
		c.JSON(http.StatusOK, response.OK(result))
	}
}

func serveOperation(operation binding) gin.HandlerFunc {
	return func(c *gin.Context) {
		actor, _ := c.Request.Context().Value(actorKey{}).(identity)
		if operation.Effect == automationcontract.EffectWrite && (c.GetHeader(automationcontract.InstanceHeader) != actor.instanceID || c.GetHeader(automationcontract.ContractHeader) != operation.Revision()) {
			apiError(c, &operationError{http.StatusConflict, "CONTRACT_MISMATCH", "Refresh the operation contract and verify the target instance before writing."})
			return
		}
		input, err := readAPIInput(c, operation.Operation)
		if err != nil {
			apiError(c, invalidArguments(err))
			return
		}
		result, failure := operation.execute(c.Request.Context(), actor, input)
		if failure != nil {
			apiError(c, failure)
			return
		}
		c.JSON(http.StatusOK, response.OK(result))
	}
}

func readAPIInput(c *gin.Context, operation automationcontract.Operation) (map[string]string, error) {
	values, err := url.ParseQuery(c.Request.URL.RawQuery)
	if err != nil || duplicateValues(values) {
		return nil, errors.New("invalid or duplicate query argument")
	}
	input := make(map[string]string, len(values)+len(c.Params))
	queryFields := make(map[string]bool)
	body := automationcontract.Operation{}
	for _, param := range operation.Parameters {
		switch param.In {
		case "query":
			queryFields[param.Name] = true
		case automationcontract.LocationBody:
			body.Parameters = append(body.Parameters, param)
		}
	}
	for key, items := range values {
		if !queryFields[key] {
			return nil, fmt.Errorf("unknown query argument %q", key)
		}
		input[key] = items[0]
	}
	for _, param := range c.Params {
		input[param.Key] = param.Value
	}
	if len(body.Parameters) == 0 {
		if c.Request.ContentLength != 0 {
			return nil, errors.New("this operation does not accept a body")
		}
		return input, nil
	}
	media, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
	if err != nil || media != "application/json" {
		return nil, errors.New("Content-Type must be application/json")
	}
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, maxOperationBody))
	if err != nil || len(raw) == 0 {
		return nil, errors.New("a JSON body of at most 128 KiB is required")
	}
	fields, err := body.JSONInput(raw)
	if err != nil {
		return nil, err
	}
	for key, value := range fields {
		input[key] = value
	}
	return input, nil
}

func apiError(c *gin.Context, failure *operationError) {
	c.Header(errorCodeHeader, failure.Code)
	response.AbortWithError(c, failure.Status, failure.Message)
}

// MCPHandler serves Streamable HTTP and advertises only the granted operations.
func MCPHandler() gin.HandlerFunc {
	operations := bindings()
	transport := mcp.NewStreamableHTTPHandler(func(request *http.Request) *mcp.Server {
		actor, _ := request.Context().Value(actorKey{}).(identity)
		return mcpServer(actor, operations)
	}, &mcp.StreamableHTTPOptions{
		Stateless: true, JSONResponse: true, MaxRequestBodyBytes: maxOperationBody,
		DisableLocalhostProtection: true,
	})
	return withInstance(func(c *gin.Context, site instance) {
		origin := c.GetHeader("Origin")
		if !site.matchesMCPHost(c.Request) || origin != "" && origin != site.origin {
			c.Status(http.StatusForbidden)
			return
		}
		actor, failure := authorizeTransport(c, site, true)
		if failure != nil {
			c.Status(failure.Status)
			return
		}
		request := c.Request.WithContext(context.WithValue(c.Request.Context(), actorKey{}, actor))
		transport.ServeHTTP(c.Writer, request)
	})
}

func (site instance) matchesMCPHost(request *http.Request) bool {
	scheme, _, _ := strings.Cut(site.origin, "://")
	origin, err := automationcontract.HTTPOrigin(scheme + "://" + request.Host)
	if err == nil && origin == site.origin {
		return true
	}

	// The local Next.js proxy changes Host and supplies X-Forwarded-Host.
	// Accept this only when the public address, upstream and direct peer are loopback.
	peer, _, err := net.SplitHostPort(request.RemoteAddr)
	if scheme != schemeHTTP || err != nil || !net.ParseIP(peer).IsLoopback() {
		return false
	}
	forwarded, err := automationcontract.HTTPOrigin(schemeHTTP + "://" + request.Header.Get("X-Forwarded-Host"))
	return err == nil && forwarded == site.origin && loopbackOrigin(origin) && loopbackOrigin(forwarded)
}

func loopbackOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Hostname() == "localhost" || net.ParseIP(parsed.Hostname()).IsLoopback())
}

func mcpServer(actor identity, operations []binding) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "OpenFlare", Version: buildinfo.Version}, nil)
	for _, operation := range operations {
		if !operation.permitted(actor) {
			continue
		}
		readOnly := operation.Effect == automationcontract.EffectRead
		destructive := !readOnly
		server.AddTool(&mcp.Tool{
			Name: operation.ID, Description: operation.Description,
			InputSchema: operation.InputSchema(), OutputSchema: operation.Output,
			Annotations: &mcp.ToolAnnotations{ReadOnlyHint: readOnly, DestructiveHint: &destructive, IdempotentHint: readOnly},
		}, mcpOperation(operation, actor))
	}
	return server
}

func mcpOperation(operation binding, actor identity) mcp.ToolHandler {
	return func(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		input, err := operation.JSONInput(request.Params.Arguments)
		if err != nil {
			return mcpOperationError(ctx, invalidArguments(err)), nil
		}
		result, failure := operation.execute(ctx, actor, input)
		if failure != nil {
			return mcpOperationError(ctx, failure), nil
		}
		encoded, err := json.Marshal(result)
		if err != nil {
			return mcpOperationError(ctx, operationFailed(ctx, operation.ID, err)), nil
		}
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}, StructuredContent: result,
		}, nil
	}
}

func mcpOperationError(ctx context.Context, failure *operationError) *mcp.CallToolResult {
	id := requestID()
	encoded, err := json.Marshal(struct {
		*operationError
		RequestID string `json:"request_id"`
	}{failure, id})
	if err != nil {
		logger.ErrorF(ctx, "Automation request_id=%s error response: %v", id, err)
	}
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}}
}
