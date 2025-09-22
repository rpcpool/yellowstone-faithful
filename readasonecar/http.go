package readasonecar

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/rpcpool/yellowstone-faithful/carreader"
	splitcarfetcher "github.com/rpcpool/yellowstone-faithful/split-car-fetcher"
)

func NewFromURLs(urls ...string) (*MultiReader, error) {
	if len(urls) == 0 {
		return nil, errors.New("no URLs provided")
	}
	containers := make([]*Container, 0, len(urls))
	for _, formattedURL := range urls {
		container, err := OpenURL(formattedURL)
		if err != nil {
			return nil, fmt.Errorf("failed to open URL %q: %w", formattedURL, err)
		}
		if container == nil {
			return nil, fmt.Errorf("container for URL %q is nil", formattedURL)
		}
		containers = append(containers, container)
	}

	rao, err := NewMultiReaderFromContainers(containers)
	if err != nil {
		return nil, fmt.Errorf("failed to create multi reader from %v: %w", urls, err)
	}
	return rao, nil
}

func OpenURL(url string) (*Container, error) {
	rfspc, byteLen, err := splitcarfetcher.NewRemoteHTTPFileAsIoReaderAt(
		context.Background(),
		url,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create remote file split car reader from %q: %w", url, err)
	}
	sr := io.NewSectionReader(rfspc, 0, byteLen) // *io.SectionReader
	cr, err := carreader.NewPrefetching(io.NopCloser(sr))
	if err != nil {
		return nil, fmt.Errorf("failed to create car reader from %q: %w", url, err)
	}

	headerSize, err := cr.HeaderSize()
	if err != nil {
		return nil, fmt.Errorf("failed to get header size from %q: %w", url, err)
	}
	container := &Container{
		Path:       url,
		Size:       uint64(byteLen),
		HeaderSize: headerSize,
		CarReader:  cr,
	}
	return container, nil
}
