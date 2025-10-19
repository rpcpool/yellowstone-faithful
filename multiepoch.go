package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/goware/urlx"
	"github.com/libp2p/go-reuseport"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/rpcpool/yellowstone-faithful/metrics"
	old_faithful_grpc "github.com/rpcpool/yellowstone-faithful/old-faithful-proto/old-faithful-grpc"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"k8s.io/klog/v2"

	"github.com/rpcpool/yellowstone-faithful/telemetry"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Options struct {
	GsfaOnlySignatures     bool
	EpochSearchConcurrency int
	// Hot tier configuration
	HotTierLimit int // Number of most recent epochs to search in parallel (default: 30)
}

type MultiEpoch struct {
	mu      sync.RWMutex
	options *Options
	epochs  map[uint64]*Epoch
	old_faithful_grpc.UnimplementedOldFaithfulServer
}

func NewMultiEpoch(options *Options) *MultiEpoch {
	return &MultiEpoch{
		options: options,
		epochs:  make(map[uint64]*Epoch),
	}
}

func (m *MultiEpoch) GetEpoch(epoch uint64) (*Epoch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ep, ok := m.epochs[epoch]
	if !ok {
		return nil, fmt.Errorf("epoch %d not found", epoch)
	}
	return ep, nil
}

func (m *MultiEpoch) HasEpoch(epoch uint64) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.epochs[epoch]
	return ok
}

func (m *MultiEpoch) AddEpoch(epoch uint64, ep *Epoch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.epochs[epoch]; ok {
		return fmt.Errorf("epoch %d already exists", epoch)
	}
	m.epochs[epoch] = ep
	return nil
}

func (m *MultiEpoch) RemoveEpoch(epoch uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.epochs[epoch]; !ok {
		return fmt.Errorf("epoch %d not found", epoch)
	}
	delete(m.epochs, epoch)
	return nil
}

func (m *MultiEpoch) RemoveEpochByConfigFilepath(configFilepath string) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for epoch, ep := range m.epochs {
		if ep.config.ConfigFilepath() == configFilepath {
			ep.Close()
			delete(m.epochs, epoch)
			return epoch, nil
		}
	}
	return 0, fmt.Errorf("epoch not found for config file %q", configFilepath)
}

func (m *MultiEpoch) ReplaceEpoch(epoch uint64, ep *Epoch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.epochs[epoch]; !ok {
		return fmt.Errorf("epoch %d not found", epoch)
	}
	m.epochs[epoch] = ep
	return nil
}

func (m *MultiEpoch) ReplaceOrAddEpoch(epoch uint64, ep *Epoch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	// if the epoch already exists, close it
	if oldEp, ok := m.epochs[epoch]; ok {
		oldEp.Close()
	}
	m.epochs[epoch] = ep
	return nil
}

func (m *MultiEpoch) HasEpochWithSameHashAsFile(filepath string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, ep := range m.epochs {
		if ep.config.IsSameHashAsFile(filepath) {
			return true
		}
	}
	return false
}

func (m *MultiEpoch) CountEpochs() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.epochs)
}

// GetEpochNumbers returns a list of epoch numbers, sorted from most recent to oldest.
func (m *MultiEpoch) GetEpochNumbers() []uint64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var epochNumbers []uint64
	for epochNumber := range m.epochs {
		epochNumbers = append(epochNumbers, epochNumber)
	}
	sort.Slice(epochNumbers, func(i, j int) bool {
		return epochNumbers[i] > epochNumbers[j]
	})
	return epochNumbers
}

func (m *MultiEpoch) GetMostRecentAvailableEpoch() (*Epoch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	numbers := m.GetEpochNumbers()
	if len(numbers) > 0 {
		return m.epochs[numbers[0]], nil
	}
	return nil, fmt.Errorf("no epochs available")
}

func (m *MultiEpoch) GetOldestAvailableEpoch() (*Epoch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	numbers := m.GetEpochNumbers()
	if len(numbers) > 0 {
		return m.epochs[numbers[len(numbers)-1]], nil
	}
	return nil, fmt.Errorf("no epochs available")
}

func (m *MultiEpoch) GetFirstAvailableBlock(ctx context.Context) (*ipldbindcode.Block, error) {
	oldestEpoch, err := m.GetOldestAvailableEpoch()
	if err != nil {
		return nil, err
	}
	return oldestEpoch.GetFirstAvailableBlock(ctx)
}

func (m *MultiEpoch) GetMostRecentAvailableBlock(ctx context.Context) (*ipldbindcode.Block, error) {
	mostRecentEpoch, err := m.GetMostRecentAvailableEpoch()
	if err != nil {
		return nil, err
	}
	return mostRecentEpoch.GetMostRecentAvailableBlock(ctx)
}

func (m *MultiEpoch) GetMostRecentAvailableEpochNumber() (uint64, error) {
	numbers := m.GetEpochNumbers()
	if len(numbers) > 0 {
		return numbers[0], nil
	}
	return 0, fmt.Errorf("no epochs available")
}

func (m *MultiEpoch) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	klog.Info("Closing all epochs...")
	for _, ep := range m.epochs {
		ep.Close()
	}
	return nil
}

type ListenerConfig struct {
	ProxyConfig *ProxyConfig
}

type ProxyConfig struct {
	Target  string            `json:"target" yaml:"target"`
	Headers map[string]string `json:"headers" yaml:"headers"`
	// ProxyFailedRequests will proxy requests that fail to be handled by the local RPC server.
	ProxyFailedRequests bool `json:"proxyFailedRequests" yaml:"proxyFailedRequests"`
}

func LoadProxyConfig(configFilepath string) (*ProxyConfig, error) {
	var proxyConfig ProxyConfig
	if isJSONFile(configFilepath) {
		if err := loadFromJSON(configFilepath, &proxyConfig); err != nil {
			return nil, err
		}
	} else if isYAMLFile(configFilepath) {
		if err := loadFromYAML(configFilepath, &proxyConfig); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("config file %q must be JSON or YAML", configFilepath)
	}
	return &proxyConfig, nil
}

// ListeAndServe starts listening on the configured address and serves the RPC API.
func (m *MultiEpoch) ListenAndServe(ctx context.Context, listenOn string, lsConf *ListenerConfig) error {
	handler := newMultiEpochHandler(m, lsConf)
	handler = fasthttp.CompressHandler(handler)

	klog.Infof("RPC server listening on %s", listenOn)

	s := &fasthttp.Server{
		Handler:            handler,
		MaxRequestBodySize: 1024 * 1024,
	}
	go func() {
		// listen for context cancellation
		<-ctx.Done()
		klog.Info("RPC server shutting down...")
		defer klog.Info("RPC server shut down")
		if err := s.ShutdownWithContext(ctx); err != nil {
			klog.Errorf("Error while shutting down RPC server: %s", err)
		}
	}()
	ln, err := reuseport.Listen("tcp4", listenOn)
	if err != nil {
		klog.Fatalf("error in reuseport listener: %v", err)
		return err
	}
	return s.Serve(ln)
}

func randomRequestID() string {
	id := uuid.New().String()
	return id
}

func newMultiEpochHandler(handler *MultiEpoch, lsConf *ListenerConfig) func(ctx *fasthttp.RequestCtx) {
	// create a transparent reverse proxy
	var proxy *fasthttp.HostClient
	if lsConf != nil && lsConf.ProxyConfig != nil && lsConf.ProxyConfig.Target != "" {
		target := lsConf.ProxyConfig.Target
		parsedTargetURL, err := urlx.Parse(target)
		if err != nil {
			panic(fmt.Errorf("invalid proxy target URL %q: %w", target, err))
		}
		addr := parsedTargetURL.Hostname()
		if parsedTargetURL.Port() != "" {
			addr += ":" + parsedTargetURL.Port()
		}
		proxy = &fasthttp.HostClient{
			Addr:  addr,
			IsTLS: parsedTargetURL.Scheme == "https",
		}
		klog.Infof("Will proxy unhandled RPC methods to %q", addr)
	}
	metricsHandler := fasthttpadaptor.NewFastHTTPHandler(promhttp.Handler())
	return func(reqCtx *fasthttp.RequestCtx) {
		startedAt := time.Now()
		reqID := randomRequestID()
		var method string = "<unknown>"

		// Extract OpenTelemetry context from request headers
		propagator := otel.GetTextMapPropagator()
		headerCarrier := propagation.MapCarrier{}
		reqCtx.Request.Header.VisitAll(func(k, v []byte) {
			headerCarrier.Set(string(k), string(v))
		})
		ctx := propagator.Extract(reqCtx, headerCarrier)

		defer func() {
			if method == "/metrics" || method == "/health" {
				return
			}
			took := time.Since(startedAt)
			klog.V(2).Infof("[%s] request %q took %s", reqID, sanitizeMethod(method), took)
			metrics.StatusCode.WithLabelValues(fmt.Sprint(reqCtx.Response.StatusCode())).Inc()
			metrics.RpcResponseLatencyHistogram.WithLabelValues(sanitizeMethod(method)).Observe(took.Seconds())
			metrics.RpcResponseLatencySummary.WithLabelValues(sanitizeMethod(method)).Observe(took.Seconds())
		}()
		{
			// handle the /metrics endpoint
			if string(reqCtx.Path()) == "/metrics" {
				method = "/metrics"
				metricsHandler(reqCtx)
				return
			}
			{
				// Handle the /health endpoint
				if string(reqCtx.Path()) == "/health" && reqCtx.IsGet() {
					method = "/health"
					reqCtx.SetStatusCode(http.StatusOK)
					return
				}
			}
			{
				// handle the /api/v1/* endpoint
				if strings.HasPrefix(string(reqCtx.Path()), "/api/v1/") {
					handler.apiHandler(reqCtx)
					return
				}
			}
		}
		{
			// make sure the method is POST
			if !reqCtx.IsPost() {
				replyJSON(reqCtx, http.StatusMethodNotAllowed, jsonrpc2.Response{
					Error: &jsonrpc2.Error{
						Code:    jsonrpc2.CodeMethodNotFound,
						Message: "Method not allowed",
					},
				})
				return
			}

			// limit request body size
			if reqCtx.Request.Header.ContentLength() > 1024 {
				replyJSON(reqCtx, http.StatusRequestEntityTooLarge, jsonrpc2.Response{
					Error: &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInvalidRequest,
						Message: "Request entity too large",
					},
				})
				return
			}
		}
		// read request body
		body := reqCtx.Request.Body()

		reqCtx.Response.Header.Set("X-Request-ID", reqID)

		// parse request
		var rpcRequest jsonrpc2.Request
		if err := fasterJson.Unmarshal(body, &rpcRequest); err != nil {
			klog.Errorf("[%s] failed to parse request body: %v", err)
			replyJSON(reqCtx, http.StatusBadRequest, jsonrpc2.Response{
				Error: &jsonrpc2.Error{
					Code:    jsonrpc2.CodeParseError,
					Message: "Parse error",
				},
			})
			return
		}
		method = rpcRequest.Method
		metrics.RpcRequestByMethod.WithLabelValues(sanitizeMethod(method)).Inc()
		defer func() {
			metrics.MethodToCode.WithLabelValues(sanitizeMethod(method), fmt.Sprint(reqCtx.Response.StatusCode())).Inc()
		}()

		klog.V(2).Infof("[%s] method=%q", reqID, sanitizeMethod(method))
		klog.V(3).Infof("[%s] received request with body: %q", reqID, strings.TrimSpace(string(body)))
		// OpenTelemetry: Start a server span with the observed method
		ctx, span := telemetry.StartSpan(ctx, "jsonrpc."+method,
			trace.WithAttributes(
				attribute.String("rpc.method", method),
				attribute.String("rpc.request_id", reqID),
				attribute.String("client.address", reqCtx.RemoteAddr().String()),
			),
		)
		defer span.End()

		klog.V(2).Infof("[%s] method=%q", reqID, sanitizeMethod(method))
		klog.V(3).Infof("[%s] received request with body: %q", reqID, strings.TrimSpace(string(body)))

		if proxy != nil && !isValidLocalMethod(rpcRequest.Method) {
			klog.V(2).Infof("[%s] Unhandled method %q, proxying to %q", reqID, rpcRequest.Method, proxy.Addr)
			// proxy the request to the target
			proxyToAlternativeRPCServer(
				handler,
				lsConf,
				proxy,
				reqCtx,
				&rpcRequest,
				body,
				reqID,
			)
			metrics.MethodToNumProxied.WithLabelValues(sanitizeMethod(method)).Inc()
			return
		}

		rqCtx := &requestContext{ctx: reqCtx}

		if method == "getVersion" {
			versionInfo := make(map[string]any)
			faithfulVersion := handler.GetFaithfulVersionInfo()
			versionInfo["faithful"] = faithfulVersion

			solanaVersion := handler.GetSolanaVersionInfo()
			for k, v := range solanaVersion {
				versionInfo[k] = v
			}

			err := rqCtx.Reply(
				reqCtx,
				rpcRequest.ID,
				versionInfo,
			)
			if err != nil {
				telemetry.RecordError(span, err, "Failed to reply to getVersion")
				klog.Errorf("[%s] failed to reply to getVersion: %v", reqID, err)
			}
			return
		}

		// errorResp is the error response to be sent to the client.
		errorResp, err := handler.handleRequest(setRequestIDToContext(reqCtx, reqID), rqCtx, &rpcRequest)
		if err != nil {
			telemetry.RecordError(span, err, "Failed to handle JSON-RPC request")
			klog.Errorf("[%s] failed to handle %q: %v", reqID, sanitizeMethod(method), err)
		}
		if errorResp != nil {
			metrics.MethodToSuccessOrFailure.WithLabelValues(sanitizeMethod(method), "failure").Inc()
			if proxy != nil && lsConf.ProxyConfig.ProxyFailedRequests {
				klog.Warningf("[%s] Failed local method %q, proxying to %q", reqID, rpcRequest.Method, proxy.Addr)
				// proxy the request to the target
				proxyToAlternativeRPCServer(
					handler,
					lsConf,
					proxy,
					reqCtx,
					&rpcRequest,
					body,
					reqID,
				)
				metrics.MethodToNumProxied.WithLabelValues(sanitizeMethod(method)).Inc()
				return
			} else {
				if errors.Is(err, ErrNotFound) {
					// reply with null result
					rqCtx.Reply(
						reqCtx,
						rpcRequest.ID,
						nil,
					)
				} else {
					rqCtx.ReplyWithError(
						reqCtx,
						rpcRequest.ID,
						errorResp,
					)
				}
			}
			return
		}
		metrics.MethodToSuccessOrFailure.WithLabelValues(sanitizeMethod(method), "success").Inc()
	}
}

func proxyToAlternativeRPCServer(
	handler *MultiEpoch,
	lsConf *ListenerConfig,
	proxy *fasthttp.HostClient,
	reqCtx *fasthttp.RequestCtx,
	rpcRequest *jsonrpc2.Request,
	body []byte,
	reqID string,
) {
	// proxy the request to the target
	proxyReq := fasthttp.AcquireRequest()
	defer fasthttp.ReleaseRequest(proxyReq)
	{
		for k, v := range lsConf.ProxyConfig.Headers {
			proxyReq.Header.Set(k, v)
		}
	}
	proxyReq.Header.SetMethod("POST")
	proxyReq.Header.SetContentType("application/json")
	proxyReq.SetRequestURI(lsConf.ProxyConfig.Target)
	proxyReq.SetBody(body)
	proxyResp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseResponse(proxyResp)
	if err := proxy.Do(proxyReq, proxyResp); err != nil {
		klog.Errorf("[%s] failed to proxy request: %v", reqID, err)
		replyJSON(reqCtx, http.StatusInternalServerError, jsonrpc2.Response{
			Error: &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: "Internal error",
			},
		})
		return
	}
	reqCtx.Response.Header.Set("Content-Type", "application/json")
	reqCtx.Response.SetStatusCode(proxyResp.StatusCode())
	if rpcRequest.Method == "getVersion" {
		enriched, err := handler.tryEnrichGetVersion(proxyResp.Body())
		if err != nil {
			klog.Errorf("[%s] failed to enrich getVersion response: %v", reqID, err)
			reqCtx.Response.SetBody(proxyResp.Body())
		} else {
			reqCtx.Response.SetBody(enriched)
		}
	} else {
		reqCtx.Response.SetBody(proxyResp.Body())
	}
	// TODO: handle compression.
}

func sanitizeMethod(method string) string {
	if isValidLocalMethod(method) {
		return method
	}
	return allowOnlyAsciiPrintable(method)
}

func allowOnlyAsciiPrintable(s string) string {
	return strings.Map(func(r rune) rune {
		// allow only printable ASCII characters
		if r >= 32 && r <= 126 {
			return r
		}
		return -1
	}, s)
}

func isValidLocalMethod(method string) bool {
	switch method {
	case "getBlock", "getTransaction", "getSignaturesForAddress", "getBlockTime", "getGenesisHash", "getFirstAvailableBlock", "getSlot":
		return true
	default:
		return false
	}
}

// jsonrpc2.RequestHandler interface
func (ser *MultiEpoch) handleRequest(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	switch req.Method {
	case "getBlock":
		spanCtx, span := telemetry.StartSpan(ctx, "jsonrpc.handleGetBlock")
		defer span.End()
		return ser.handleGetBlock(spanCtx, conn, req)
	case "getTransaction":
		spanCtx, span := telemetry.StartSpan(ctx, "jsonrpc.handleGetTransaction")
		defer span.End()
		return ser.handleGetTransaction(spanCtx, conn, req)
	case "getSignaturesForAddress":
		spanCtx, span := telemetry.StartSpan(ctx, "jsonrpc.handleGetSignaturesForAddress")
		defer span.End()
		return ser.handleGetSignaturesForAddress(spanCtx, conn, req)
	case "getBlockTime":
		spanCtx, span := telemetry.StartSpan(ctx, "jsonrpc.handleGetBlockTime")
		defer span.End()
		return ser.handleGetBlockTime(spanCtx, conn, req)
	case "getGenesisHash":
		spanCtx, span := telemetry.StartSpan(ctx, "jsonrpc.handleGetGenesisHash")
		defer span.End()
		return ser.handleGetGenesisHash(spanCtx, conn, req)
	case "getFirstAvailableBlock":
		spanCtx, span := telemetry.StartSpan(ctx, "jsonrpc.handleGetFirstAvailableBlock")
		defer span.End()
		return ser.handleGetFirstAvailableBlock(spanCtx, conn, req)
	case "getSlot":
		spanCtx, span := telemetry.StartSpan(ctx, "jsonrpc.handleGetSlot")
		defer span.End()
		return ser.handleGetSlot(spanCtx, conn, req)
	default:
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "Method not found",
		}, fmt.Errorf("method not found")
	}
}

func init() {
	// Add an entry to the metric with the version information.
	labeledValues := map[string]string{
		"started_at":   StartedAt.Format(time.RFC3339),
		"tag":          GitTag,
		"commit":       GitCommit,
		"compiler":     "",
		"goarch":       "",
		"goos":         "",
		"goamd64":      "",
		"vcs":          "",
		"vcs_revision": "",
		"vcs_time":     "",
		"vcs_modified": "",
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if isAnyOf(setting.Key,
				"-compiler",
				"GOARCH",
				"GOOS",
				"GOAMD64",
				"vcs",
				"vcs.revision",
				"vcs.time",
				"vcs.modified",
			) {
				switch setting.Key {
				case "-compiler":
					labeledValues["compiler"] = setting.Value
				case "GOARCH":
					labeledValues["goarch"] = setting.Value
				case "GOOS":
					labeledValues["goos"] = setting.Value
				case "GOAMD64":
					labeledValues["goamd64"] = setting.Value
				case "vcs":
					labeledValues["vcs"] = setting.Value
				case "vcs.revision":
					labeledValues["vcs_revision"] = setting.Value
				case "vcs.time":
					labeledValues["vcs_time"] = setting.Value
				case "vcs.modified":
					labeledValues["vcs_modified"] = setting.Value
				}
			}
		}
	}
	metrics.Version.With(labeledValues).Set(1)
}

var StartedAt = time.Now()

func GetUptime() time.Duration {
	return time.Since(StartedAt)
}
