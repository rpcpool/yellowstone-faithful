package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mr-tron/base58"
	sigtoepoch "github.com/rpcpool/yellowstone-faithful/sig-to-epoch"
	"github.com/sourcegraph/jsonrpc2"
	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2"
)

type Options struct {
	GsfaOnlySignatures bool
	PathToSigToEpoch   string // path to the sig-to-epoch index directory
}

type MultiEpoch struct {
	mu         sync.RWMutex
	options    *Options
	sigToEpoch *sigtoepoch.Index
	epochs     map[uint64]*Epoch
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

func (m *MultiEpoch) ReplaceEpoch(epoch uint64, ep *Epoch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.epochs[epoch]; !ok {
		return fmt.Errorf("epoch %d not found", epoch)
	}
	m.epochs[epoch] = ep
	return nil
}

func (m *MultiEpoch) CountEpochs() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.epochs)
}

func (m *MultiEpoch) EpochNumbers() []uint64 {
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

func (m *MultiEpoch) GetFirstAvailableEpoch() (*Epoch, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	numbers := m.EpochNumbers()
	if len(numbers) > 0 {
		return m.epochs[numbers[0]], nil
	}
	return nil, fmt.Errorf("no epochs available")
}

// ListeAndServe starts listening on the configured address and serves the RPC API.
func (m *MultiEpoch) ListenAndServe(listenOn string) error {
	h := newMultiEpochHandler(m)
	h = fasthttp.CompressHandler(h)

	if sigToEpochIndexDir := m.options.PathToSigToEpoch; sigToEpochIndexDir != "" {
		klog.Infof("Opening sig-to-epoch index from %s", sigToEpochIndexDir)
		index, err := sigtoepoch.NewIndex(
			sigToEpochIndexDir,
		)
		if err != nil {
			return fmt.Errorf("error while opening sig-to-epoch index: %w", err)
		}
		defer func() {
			if err := index.Close(); err != nil {
				klog.Errorf("Error while closing: %s", err)
			}
		}()
		{
			epochs, err := index.Epochs()
			if err != nil {
				return fmt.Errorf("error while getting epochs list from sig-to-epoch index: %w", err)
			}
			klog.Infof("sig-to-epoch index contains %d epochs", len(epochs))
			for _, epoch := range epochs {
				klog.Infof("- Epoch #%d", epoch)
			}
		}
		m.sigToEpoch = index
	}

	klog.Infof("RPC server listening on %s", listenOn)
	return fasthttp.ListenAndServe(listenOn, h)
}

func randomRequestID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return strings.ToUpper(base58.Encode(b))
}

func newMultiEpochHandler(handler *MultiEpoch) func(ctx *fasthttp.RequestCtx) {
	return func(c *fasthttp.RequestCtx) {
		startedAt := time.Now()
		reqID := randomRequestID()
		defer func() {
			klog.Infof("[%s] request took %s", reqID, time.Since(startedAt))
		}()
		{
			// make sure the method is POST
			if !c.IsPost() {
				replyJSON(c, http.StatusMethodNotAllowed, jsonrpc2.Response{
					Error: &jsonrpc2.Error{
						Code:    jsonrpc2.CodeMethodNotFound,
						Message: "Method not allowed",
					},
				})
				return
			}

			// limit request body size
			if c.Request.Header.ContentLength() > 1024 {
				replyJSON(c, http.StatusRequestEntityTooLarge, jsonrpc2.Response{
					Error: &jsonrpc2.Error{
						Code:    jsonrpc2.CodeInvalidRequest,
						Message: "Request entity too large",
					},
				})
				return
			}
		}
		// read request body
		body := c.Request.Body()

		// parse request
		var rpcRequest jsonrpc2.Request
		if err := json.Unmarshal(body, &rpcRequest); err != nil {
			klog.Errorf("[%s] failed to parse request body: %v", err)
			replyJSON(c, http.StatusBadRequest, jsonrpc2.Response{
				Error: &jsonrpc2.Error{
					Code:    jsonrpc2.CodeParseError,
					Message: "Parse error",
				},
			})
			return
		}

		klog.Infof("[%s] received request: %q", reqID, strings.TrimSpace(string(body)))

		rqCtx := &requestContext{ctx: c}
		method := rpcRequest.Method

		// errorResp is the error response to be sent to the client.
		errorResp, err := handler.handleRequest(c, rqCtx, &rpcRequest)
		if err != nil {
			klog.Errorf("[%s] failed to handle %s: %v", reqID, sanitizeMethod(method), err)
		}
		if errorResp != nil {
			rqCtx.ReplyWithError(
				c,
				rpcRequest.ID,
				errorResp,
			)
			return
		}
	}
}

func sanitizeMethod(method string) string {
	if isValidMethod(method) {
		return method
	}
	return "<unknown>"
}

func isValidMethod(method string) bool {
	switch method {
	case "getBlock", "getTransaction", "getSignaturesForAddress":
		return true
	default:
		return false
	}
}

// jsonrpc2.RequestHandler interface
func (ser *MultiEpoch) handleRequest(ctx context.Context, conn *requestContext, req *jsonrpc2.Request) (*jsonrpc2.Error, error) {
	switch req.Method {
	case "getBlock":
		return ser.handleGetBlock(ctx, conn, req)
	case "getTransaction":
		return ser.handleGetTransaction(ctx, conn, req)
	case "getSignaturesForAddress":
		return ser.handleGetSignaturesForAddress(ctx, conn, req)
	default:
		return &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "Method not found",
		}, fmt.Errorf("method not found")
	}
}
