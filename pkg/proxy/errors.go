package proxy

import "errors"

var (
	// Proxy errors
	ErrInvalidRequestUrl = errors.New("failed to initiate remote URL request")
	ErrCantDownloadFile  = errors.New("remote file failed to download")
)
