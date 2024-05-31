package proxy

import "errors"

var (
	// Proxy errors
	ErrExpiredUrl        = errors.New("url is expired")
	ErrInvalidSignature  = errors.New("signature is invalid")
	ErrInvalidRequestUrl = errors.New("failed to initiate remote URL request")
	ErrCantDownloadFile  = errors.New("remote file failed to download")
)
