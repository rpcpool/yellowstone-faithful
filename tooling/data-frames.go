package tooling

import (
	"bytes"
	"context"
	"fmt"
	"sort"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
)

func LoadDataFromDataFrames(
	firstDataFrame *ipldbindcode.DataFrame,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) ([]byte, error) {
	allFrames, err := getAllFramesFromDataFrame(firstDataFrame, dataFrameGetter)
	if err != nil {
		return nil, err
	}
	expectedTotal, ok := firstDataFrame.GetTotal()
	if ok {
		if len(allFrames) != expectedTotal {
			return nil, fmt.Errorf("expected %d frames, got %d", expectedTotal, len(allFrames))
		}
		// If firstDataFrame does not have a total, it means it is the only frame.
	}
	dataBuffer := new(bytes.Buffer)
	for i := range allFrames {
		dataBuffer.Write(allFrames[i].Bytes())
	}
	// verify the data hash (if present)
	bufHash, ok := firstDataFrame.GetHash()
	if !ok {
		return dataBuffer.Bytes(), nil
	}
	err = ipldbindcode.VerifyHash(dataBuffer.Bytes(), bufHash)
	if err != nil {
		return nil, err
	}
	return dataBuffer.Bytes(), nil
}

func getAllFramesFromDataFrame(
	firstDataFrame *ipldbindcode.DataFrame,
	dataFrameGetter func(ctx context.Context, wantedCid cid.Cid) (*ipldbindcode.DataFrame, error),
) ([]*ipldbindcode.DataFrame, error) {
	frames := []*ipldbindcode.DataFrame{firstDataFrame}
	// get the next data frames
	next, ok := firstDataFrame.GetNext()
	if !ok || len(next) == 0 {
		return frames, nil
	}
	for _, cid := range next {
		nextDataFrame, err := dataFrameGetter(context.Background(), cid.(cidlink.Link).Cid)
		if err != nil {
			return nil, err
		}
		nextFrames, err := getAllFramesFromDataFrame(nextDataFrame, dataFrameGetter)
		if err != nil {
			return nil, err
		}
		frames = append(frames, nextFrames...)
	}

	// Order the frames by index
	sort.Slice(frames, func(i, j int) bool {
		iIndex, iOk := frames[i].GetIndex()
		jIndex, jOk := frames[j].GetIndex()
		if !iOk || !jOk {
			return iOk
		}
		return iIndex < jIndex
	})
	return frames, nil
}
