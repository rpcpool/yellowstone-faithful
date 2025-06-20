package splitcarfetcher

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/filecoin-project/go-address"
	"github.com/ipfs/go-cid"
	"k8s.io/klog/v2"
)

// provider,deal_uuid,file_name,url,commp_piece_cid,file_size,padded_size,payload_cid
type Deal struct {
	Provider       address.Address
	DealUUID       string
	FileName       string
	URL            string
	CommpPieceCID  cid.Cid
	FileSize       int64
	PaddedFileSize int64
	PayloadCID     string
}

type DealRegistry struct {
	pieceToDeal map[cid.Cid]Deal
}

func NewDealRegistry() *DealRegistry {
	return &DealRegistry{
		pieceToDeal: make(map[cid.Cid]Deal),
	}
}

const (
	requiredFields = 8 // Number of required fields in the CSV file
)

// DealsFromCSV reads a CSV file and returns a DealRegistry.
func DealsFromCSV(path string) (*DealRegistry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", path, err)
	}
	defer file.Close()

	r := csv.NewReader(file)
	r.FieldsPerRecord = -1 // Allow variable number of fields
	r.Comment = '#'
	r.TrimLeadingSpace = true

	registry := NewDealRegistry()

	// read header
	if header, err := r.Read(); err != nil {
		return registry, err
	} else {
		// check that the header has at least the required fields
		if len(header) < 8 ||
			header[0] != "provider" ||
			header[1] != "deal_uuid" ||
			header[2] != "file_name" ||
			header[3] != "url" ||
			header[4] != "commp_piece_cid" ||
			header[5] != "file_size" ||
			header[6] != "padded_size" ||
			header[7] != "payload_cid" {
			return registry, fmt.Errorf("invalid header: %v", header)
		}
	}
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return registry, fmt.Errorf("failed to read csv record line: %w", err)
		}

		// Ensure we have at least the required number of fields
		if len(record) < requiredFields {
			return registry, fmt.Errorf("record has insufficient fields: %d, expected at least %d", len(record), requiredFields)
		}

		maddr, err := address.NewFromString(record[0])
		if err != nil {
			return registry, fmt.Errorf("failed to parse miner address: %w", err)
		}

		fileSize, err := strconv.ParseInt(record[5], 10, 64)
		if err != nil {
			return registry, fmt.Errorf("failed to parse file_size: %w", err)
		}

		paddedFileSize, err := strconv.ParseInt(record[6], 10, 64)
		if err != nil {
			return registry, fmt.Errorf("failed to parse padded_size: %w", err)
		}

		deal := Deal{
			Provider:       maddr,
			DealUUID:       record[1],
			FileName:       record[2],
			URL:            record[3],
			CommpPieceCID:  cid.MustParse(record[4]),
			FileSize:       fileSize,
			PaddedFileSize: paddedFileSize,
			PayloadCID:     record[7],
		}

		// if the same piece CID is associated with multiple deals, the last one wins, but print a warning
		if _, ok := registry.pieceToDeal[deal.CommpPieceCID]; ok {
			klog.Warningf("WARNING: piece CID %s is associated with multiple deals, the last one wins\n", deal.CommpPieceCID)
		}

		registry.pieceToDeal[deal.CommpPieceCID] = deal
	}

	return registry, nil
}

// GetDeal returns the deal associated with the given piece CID.
func (r *DealRegistry) GetDeal(pieceCID cid.Cid) (Deal, bool) {
	deal, ok := r.pieceToDeal[pieceCID]
	return deal, ok
}

// GetMinerByPieceCID returns the miner associated with the given piece CID.
func (r *DealRegistry) GetMinerByPieceCID(pieceCID cid.Cid) (address.Address, bool) {
	deal, ok := r.pieceToDeal[pieceCID]
	if !ok {
		return address.Address{}, false
	}
	return deal.Provider, true
}

// GetAllDeals returns a copy of all deals in the registry as a map from piece CID to Deal.
func (r *DealRegistry) GetAllDeals() map[cid.Cid]Deal {
	copyMap := make(map[cid.Cid]Deal, len(r.pieceToDeal))
	for k, v := range r.pieceToDeal {
		copyMap[k] = v
	}
	return copyMap
}
