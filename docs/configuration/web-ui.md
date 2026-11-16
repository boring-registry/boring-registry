# Web UI

The boring-registry can serve a read-only web UI under `/ui/` to browse the modules, providers and mirrored providers in the storage backend.
Module pages render the `README.md` and list the inputs and outputs declared in the root `.tf` files of the module archive.
Provider pages list versions, platforms and signing keys.

You can activate the web UI by using the `--web-ui` flag or by setting the `BORING_REGISTRY_WEB_UI=true` environment variable.
When enabled, requests to `/` are redirected to `/ui/`.

The storage listing is cached for one minute by default.
The duration can be changed with `--web-ui-cache-ttl` or `BORING_REGISTRY_WEB_UI_CACHE_TTL`, and `0` disables caching.

***Note :** The web UI does not enforce the configured authentication. Restrict access to `/ui/` externally, for example at the ingress or network level.*
