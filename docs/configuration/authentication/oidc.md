# OIDC (OpenID Connect)

[OpenID Connect (OIDC)](https://openid.net/specs/openid-connect-core-1_0.html) enables token-based access to the boring-registry on top of [OAuth 2.0](https://datatracker.ietf.org/doc/html/rfc6749) by delegating authentication to an external Identity Provider (IdP).
This document provides instructions for configuring OIDC to authorize to the boring-registry with `teraform login` or `tofu login` using the [server-side login protocol](https://opentofu.org/docs/internals/login-protocol/)

Any generic OIDC compatible provider such as [Authelia](https://github.com/authelia/authelia), [Authentik](https://github.com/goauthentik/authentik), [Keycloak](https://github.com/keycloak/keycloak), Okta, or [Zitadel](https://github.com/zitadel/zitadel) should work.

## Overview

The boring-registry needs to be configured to redirect the CLI client to the OIDC provider through [remote service discovery](https://opentofu.org/docs/internals/remote-service-discovery/) and the [server-side login protocol](https://opentofu.org/docs/internals/login-protocol/).
The client then retrieves an OIDC token and saves it locally.
This token is then passed along requests to the boring-registry, where the token is validated and consequently provides access to authorized clients.

## Configuration

The following configuration options are available:

|Flag|Environment Variable|Description|
|---|---|---|
|`--auth-oidc-clientid`|`BORING_REGISTRY_AUTH_OIDC_CLIENTID`|OIDC client identifier|
|`--auth-oidc-issuer`|`BORING_REGISTRY_AUTH_OIDC_ISSUER`|OIDC issuer URL|
|`--auth-oidc-scopes`|`BORING_REGISTRY_AUTH_OIDC_SCOPES`|List of OAuth2 scopes|
|`--login-grant-types`|`BORING_REGISTRY_LOGIN_GRANT_TYPES`|An array describing a set of OAuth 2.0 grant types (default `[authz_code]`)|
|`--login-ports`|`BORING_REGISTRY_LOGIN_PORTS`|Inclusive range of TCP ports that the Terraform/OpenTofu CLI may use (default `[10000,10010]`)|

The remote service discovery resource can be verified after configuring OIDC with:
```json
$ curl -s https://boring-registry.example.com:5601/.well-known/terraform.json | jq
{
  "login.v1": {
    "client": "boring-registry",
    "grant_types": [
      "authz_code"
    ],
    "authz": "https://idp.example.com/oauth2/boring-registry/v1/authorize",
    "token": "https://idp.example.com/oauth2/boring-registry/v1/token",
    "ports": [
      10000,
      10010
    ],
  },
  "modules.v1": "/v1/modules/",
  "providers.v1": "/v1/providers/"
}
```

Once a login was performed with `terraform login boring-registry.example.com:5601` or `tofu login boring-registry.example.com:5601`, the OIDC token is stored locally.
With a command similar to `jq -r '.credentials."boring-registry.example.com:5601".token' ~/.terraform.d/credentials.tfrc.json`, the token can be inspected.

To aid debugging, the resulting JWT token can be inspected for example at [jwt.io](https://jwt.io/).

## Authentik

As the readers are most-likely familiar with Terraform, an example configuration for Authentik is given using the [Authentik provider](https://github.com/goauthentik/terraform-provider-authentik).

```hcl
data "authentik_flow" "default-authentication-flow" {
  slug = "default-authentication-flow"
}
data "authentik_flow" "default-authorization-flow" {
  slug = "default-provider-authorization-implicit-consent"
}

data "authentik_flow" "default-invalidation-flow" {
  slug = "default-provider-invalidation-flow"
}

data "authentik_certificate_key_pair" "generated" {
  name = "authentik Self-signed Certificate"
}

resource "authentik_provider_oauth2" "boring-registry" {
  name        = "boring-registry"
  client_id   = "boring-registry"
  client_type = "public"

  authentication_flow   = data.authentik_flow.default-authentication-flow.id
  authorization_flow    = data.authentik_flow.default-authorization-flow.id
  invalidation_flow     = data.authentik_flow.default-invalidation-flow.id
  access_token_validity = "hours=2"
  allowed_redirect_uris = [
    {
      matching_mode = "regex",
      url           = "http:\\/\\/localhost:\\d+\\/login",
    }
  ]
  signing_key = data.authentik_certificate_key_pair.generated.id
}

resource "authentik_application" "boring-registry" {
  name              = "boring-registry"
  slug              = "boring-registry"
  protocol_provider = authentik_provider_oauth2.boring-registry.id
}
```

The boring-registry is configured with the following flags.
Some flags were left out for clarity:

```sh
boring-registry server \
    --auth-oidc-clientid=boring-registry \
    --auth-oidc-issuer=https://authentik.example.com/application/o/boring-registry/
```

## Okta

The boring-registry has provided unofficial and undocumented support for Okta for years, but the Okta-specific implementation is deprecated in favor of the generic OIDC implementation.
The following documentation shows how to integrate Okta with the generic OIDC implementation.

As the readers are most-likely familiar with Terraform, an example configuration for Okta is given using the [Okta provider](https://github.com/okta/terraform-provider-okta).

```hcl
resource "okta_app_oauth" "boring-registry" {
  type           = "browser"
  label          = "boring-registry"
  consent_method = "TRUSTED"
  redirect_uris = [
    for port in range(10000, 10011) : "http://localhost:${port}/login"
  ]
  grant_types = [
    "authorization_code",
  ]
  response_types = [
    "code",
  ]
  token_endpoint_auth_method = "none"
}

resource "okta_auth_server" "boring-registry" {
  audiences   = [okta_app_oauth.boring-registry.client_id]
  description = "Auth server for boring-registry"
  name        = "boring-registry"
  issuer_mode = "ORG_URL"
}

resource "okta_auth_server_policy" "all" {
  auth_server_id   = okta_auth_server.boring-registry.id
  name             = "all-access"
  priority         = 1
  description      = "Provide everyone access" # This is just an example!
  client_whitelist = [okta_app_oauth.boring-registry.id]
}

resource "okta_auth_server_policy_rule" "all_access" {
  auth_server_id                = okta_auth_server.boring-registry.id
  policy_id                     = okta_auth_server_policy.all.id
  name                          = "all-access"
  priority                      = 1
  access_token_lifetime_minutes = 120
  grant_type_whitelist          = ["authorization_code"]
  group_whitelist               = ["EVERYONE"]
  scope_whitelist               = ["*"]
}

output "issuer" {
  value = okta_auth_server.boring-registry.issuer
}

output "client_id" {
  value = okta_app_oauth.boring-registry.client_id
}
```

The boring-registry is then configured with the following flags.
Some flags were left out for clarity:

```sh
boring-registry server  \
    --auth-oidc-clientid=0oamy5gronmOjHTiR5d7 \
    --auth-oidc-issuer=https://dev-05229648.okta.com/oauth2/ausmy5p653yTgxZqb5d7 \
    --auth-oidc-scopes=openid
```

The `--auth-oidc-scopes=openid` flag is provided, as Okta otherwise complains that no scopes were passed in the initial request.
