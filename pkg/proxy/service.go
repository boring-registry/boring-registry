package proxy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// ProxyUrlService represents Boring tool to manage proxyfied downloads.
type ProxyUrlService interface {
	IsProxyEnabled(ctx context.Context) bool
	GetSignedUrl(ctx context.Context, downloadUrl string, rootUrl string) (string, error)
	CheckSignedUrl(ctx context.Context, signature string, expiry int64, downloadUrl string) bool
	IsExpired(ctx context.Context, expiry int64) bool
}

type proxyUrlService struct {
	IsEnabled bool
	ProxyPath string
	HmacKey   string
	Expiry    time.Duration
}

// NewProxyUrlService returns a fully initialized Proxy.
func NewProxyUrlService(isEnabled bool, proxyPath, hmacKey string, expiry time.Duration) ProxyUrlService {
	return &proxyUrlService{
		IsEnabled: isEnabled,
		ProxyPath: proxyPath,
		HmacKey:   hmacKey,
		Expiry:    expiry,
	}
}

func (p *proxyUrlService) IsProxyEnabled(ctx context.Context) bool {
	return p.IsEnabled
}

func (p *proxyUrlService) GetSignedUrl(ctx context.Context, downloadUrl string, rootUrl string) (string, error) {
	expirationTimeStamp := time.Now().Add(p.Expiry * time.Second).Unix()
	expirationTime := fmt.Sprint(expirationTimeStamp)
	signature := p.computeSignature(downloadUrl, expirationTime)
	encodedUrl := base64.StdEncoding.EncodeToString([]byte(downloadUrl))

	finalUrl := fmt.Sprintf("%s%s/%s/%s/%s", rootUrl, p.ProxyPath, signature, expirationTime, encodedUrl)
	return finalUrl, nil
}

func (p *proxyUrlService) CheckSignedUrl(ctx context.Context, signature string, expiry int64, downloadUrl string) bool {
	expirationTime := fmt.Sprint(expiry)
	signatureComputed := p.computeSignature(downloadUrl, expirationTime)

	return signatureComputed == signature
}

func (p *proxyUrlService) IsExpired(ctx context.Context, expiry int64) bool {
	currentTimeStamp := time.Now().Unix()

	return currentTimeStamp > expiry
}

func (p *proxyUrlService) computeSignature(url, expiry string) string {
	concat := url + expiry
	data := []byte(concat)

	// Create a new HMAC by defining the hash type and using the configured key
	hmac := hmac.New(sha256.New, []byte(p.HmacKey))

	// Compute the HMAC
	hmac.Write([]byte(data))
	dataHmac := hmac.Sum(nil)

	// And export it as HEX string
	hmacHex := hex.EncodeToString(dataHmac)
	return hmacHex
}
