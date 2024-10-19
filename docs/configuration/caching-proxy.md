# Caching Proxy

if running the boring-registry in Kubernetes, the Helm chart contains an option to activate a Nginx cache sidecar.

The cache is useful for systems where a lot of stacks query module or provider versions which will trigger a scan of storage backend.
This feature came into being as a user experienced S3 throtteling. Once you reach this stage, this cache might help.

The cache will be organised based on the `Authorisation` header and is thus only really useful for API keys but not Okta/JWT. 

## Nginx configuration
The Nginx is configured in the file `templates/cache_config.yaml` of the chart.

To keep the confidentiality of the data, the cache configuration will
- bypass the cache for yet unknown `Authorisation` headers
- if successful, cache the response based on the header so re-occuring requests can be served directly

```nginx
http {
  proxy_cache_path /var/cache/nginx levels=1:2 keys_zone=my_cache:10m max_size={{- .Values.cachingProxy.cache.maxSize }}m inactive=60m use_temp_path=off;

  server {
    listen 80;

    location / {
      access_log off;
      proxy_cache my_cache;

      # Cache validity settings
      proxy_cache_valid 200 302 {{ .Values.cachingProxy.cache.hit }};
      proxy_cache_valid 204 307 {{ .Values.cachingProxy.cache.download }}; # br gives back signed URLs with `X-Amz-Expires=300` for S3
      proxy_cache_valid 404 {{ .Values.cachingProxy.cache.miss }};
      proxy_cache_revalidate on;

      # Ensure that all authorization headers are checked by the backend
      proxy_cache_bypass $http_authorization;
      proxy_set_header Authorization $http_authorization;

      # Use cache even if authorisation header is present, but key off header
      proxy_cache_key "$http_authorization$scheme$proxy_host$request_uri";

      # Pass the request to the backend server
      proxy_pass http://localhost:{{ .Values.server.port }};
    }
  }
}
```