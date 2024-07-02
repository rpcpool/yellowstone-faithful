package main

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/gagliardetto/solana-go"
	"github.com/rpcpool/yellowstone-faithful/compactindexsized"
	"github.com/valyala/fasthttp"
)

func (multi *MultiEpoch) apiHandler(reqCtx *fasthttp.RequestCtx) {
	if !reqCtx.IsGet() {
		reqCtx.SetStatusCode(fasthttp.StatusMethodNotAllowed)
		return
	}
	// Add a CLI command that takes a config file (or a command line argument) pointing at slot-to-cid and sig-to-cid and looks up the CID for a given block or transaction.
	// slot-to-cid API endpoint: /api/v1/slot-to-cid/{slot}
	// sig-to-cid API endpoint: /api/v1/sig-to-cid/{sig}
	// The API should return the CID as a string.
	// The API should return a 404 if the slot or sig is not found.
	// The API should return a 500 if there is an internal error.
	// The API should return a 400 if the slot or sig is invalid.
	// The API should return a 200 if the CID is found.

	if strings.HasPrefix(string(reqCtx.Path()), "/api/v1/slot-to-cid/") {
		slotStr := string(reqCtx.Path())[len("/api/v1/slot-to-cid/"):]
		slotStr = strings.TrimRight(slotStr, "/")
		// try to parse the slot as uint64
		slot, err := strconv.ParseUint(slotStr, 10, 64)
		if err != nil {
			reqCtx.SetStatusCode(fasthttp.StatusBadRequest)
			return
		}
		// find the epoch that contains the requested slot
		epochNumber := CalcEpochForSlot(slot)
		epochHandler, err := multi.GetEpoch(epochNumber)
		if err != nil {
			reqCtx.SetStatusCode(fasthttp.StatusNotFound) // TODO: this means epoch is not available, and probably should be another dedicated status code
			return
		}

		blockCid, err := epochHandler.FindCidFromSlot(context.TODO(), slot)
		if err != nil {
			if errors.Is(err, compactindexsized.ErrNotFound) {
				reqCtx.SetStatusCode(fasthttp.StatusNotFound)
			} else {
				reqCtx.SetStatusCode(fasthttp.StatusInternalServerError)
			}
			return
		}
		reqCtx.SetStatusCode(fasthttp.StatusOK)
		reqCtx.SetBodyString(blockCid.String())
		return
	}
	if strings.HasPrefix(string(reqCtx.Path()), "/api/v1/sig-to-cid/") {
		sigStr := string(reqCtx.Path())[len("/api/v1/sig-to-cid/"):]
		sigStr = strings.TrimRight(sigStr, "/")
		// parse the signature
		sig, err := solana.SignatureFromBase58(sigStr)
		if err != nil {
			reqCtx.SetStatusCode(fasthttp.StatusBadRequest)
			return
		}
		epochNumber, err := multi.findEpochNumberFromSignature(context.TODO(), sig)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				reqCtx.SetStatusCode(fasthttp.StatusNotFound)
			} else {
				reqCtx.SetStatusCode(fasthttp.StatusInternalServerError)
			}
			return
		}

		epochHandler, err := multi.GetEpoch(uint64(epochNumber))
		if err != nil {
			reqCtx.SetStatusCode(fasthttp.StatusNotFound) // TODO: this means epoch is not available, and probably should be another dedicated status code
			return
		}

		transactionCid, err := epochHandler.FindCidFromSignature(context.TODO(), sig)
		if err != nil {
			if errors.Is(err, compactindexsized.ErrNotFound) {
				reqCtx.SetStatusCode(fasthttp.StatusNotFound)
			} else {
				reqCtx.SetStatusCode(fasthttp.StatusInternalServerError)
			}
			return
		}
		reqCtx.SetStatusCode(fasthttp.StatusOK)
		reqCtx.SetBodyString(transactionCid.String())
		return
	}
	reqCtx.SetStatusCode(fasthttp.StatusNotFound)
}
