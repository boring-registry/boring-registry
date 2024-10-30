# Caching Proxy

if running the boring-registry in Kubernetes, the Helm chart includes an option to activate a Nginx cache sidecar.

The cache is useful for systems where a lot of stacks query module or provider versions which will trigger a scan of storage backend.
This feature came into being as a user experienced S3 throtteling. Once you reach this stage, this cache might help.

The cache will be organised based on the `Authorisation` header and is thus only really useful for API keys but not Okta/JWT.

Please note: we do not cache download URLs (HTTP codes `204` and `307`)

## Nginx configuration
The Nginx is configured in the file `templates/cache_config.yaml` of the chart.

To keep the confidentiality of the data, the cache configuration will
- bypass the cache for yet unknown `Authorisation` headers
- if successful, cache the response based on the header so re-occuring requests can be served directly

```nginx
http {
  proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=my_cache:10m max_size={{- .Values.cachingProxy.cache.maxSize }} inactive=60m use_temp_path=off;

  [ ... ]

  server {
    listen 80;

    [ ... ]
    
    location / {
      access_log off;
      proxy_cache my_cache;

      # Cache validity settings
      proxy_cache_valid 200 302 {{ .Values.cachingProxy.cache.hit }};
      proxy_cache_valid 404 {{ .Values.cachingProxy.cache.miss }};

      # Cache key including the sanitized authorization header
      proxy_cache_key "$host$request_uri$sanitized_auth";

      # Ignore cache control headers from backend
      proxy_ignore_headers Cache-Control Expires;

      # Revalidate cache on stale
      proxy_cache_revalidate on;

      # Pass all authorization headers to the backend
      proxy_set_header Authorization $http_authorization;

      # Retry mechanism settings
      proxy_next_upstream error timeout invalid_header http_500 http_502 http_503 http_504;
      proxy_next_upstream_tries 3;     # Maximum number of retries
      proxy_next_upstream_timeout 10s; # Timeout for each retry

      # Pass the request to the backend server
      proxy_pass http://localhost:{{ .Values.server.port }};
    }
    
    [ ... ]
  }
}
```