package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/sourcegraph/jsonrpc2"
	"github.com/valyala/fasthttp"
	"k8s.io/klog/v2"
)

type Options struct {
	GsfaOnlySignatures bool
}

type MultiEpoch struct {
	mu      sync.RWMutex
	options *Options
	epochs  map[uint64]*Epoch
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

// ListeAndServe starts listening on the configured address and serves the RPC API.
func (m *MultiEpoch) ListenAndServe(listenOn string) error {
	h := newMultiEpochHandler(m)
	h = fasthttp.CompressHandler(h)

	klog.Infof("RPC server listening on %s", listenOn)
	return fasthttp.ListenAndServe(listenOn, h)
}

func newMultiEpochHandler(handler *MultiEpoch) func(ctx *fasthttp.RequestCtx) {
	return func(c *fasthttp.RequestCtx) {
		startedAt := time.Now()
		defer func() {
			klog.Infof("Request took %s", time.Since(startedAt))
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
			klog.Errorf("failed to unmarshal request: %v", err)
			replyJSON(c, http.StatusBadRequest, jsonrpc2.Response{
				Error: &jsonrpc2.Error{
					Code:    jsonrpc2.CodeParseError,
					Message: "Parse error",
				},
			})
			return
		}

		klog.Infof("Received request: %q", string(body))

		rqCtx := &requestContext{ctx: c}

		// errorResp is the error response to be sent to the client.
		errorResp, err := handler.handleRequest(c, rqCtx, &rpcRequest)
		if err != nil {
			klog.Errorf("failed to handle request: %v", err)
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
		}, nil
	}
}
