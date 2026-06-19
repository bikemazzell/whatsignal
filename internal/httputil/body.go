package httputil

import (
	"fmt"
	"io"
)

func ReadLimitedBody(body io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response body exceeds maximum size (%d bytes)", maxBytes)
	}
	return data, nil
}
