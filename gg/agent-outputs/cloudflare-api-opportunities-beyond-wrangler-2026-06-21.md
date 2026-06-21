# Cloudflare API Opportunities Beyond Wrangler

Date: 2026-06-21

## Scope

This memo looks for Cloudflare API capabilities that are useful for agents and automation, but are not already well-covered by Wrangler.

What I treated as "already covered well enough by Wrangler":

- Workers lifecycle: `deploy`, `dev`, `versions`, `tail`
- Developer-platform storage and compute primitives: D1, KV, R2, Queues, Hyperdrive, Vectorize, VPC, Workflows, Pipelines, Containers, Secrets Store
- Basic Tunnel management now exposed in Wrangler's `tunnel` command set

Source basis:

- Wrangler commands index: <https://developers.cloudflare.com/workers/wrangler/commands/>
- Cloudflare API reference and product docs for the candidate areas below

## Ranking

## 1. GraphQL analytics query runner and saved query library

### What raw Cloudflare API enables

Cloudflare exposes a single GraphQL endpoint for account- and zone-scoped analytics, with access to many datasets across HTTP traffic, firewall events, Workers metrics, Access login events, network analytics, and more.

- Endpoint: `POST /client/v4/graphql`
- Query shape:

```json
{
  "query": "query($zoneTag: string, $filter: ...) { viewer { zones(filter: { zoneTag: $zoneTag }) { ... } } }",
  "variables": {
    "zoneTag": "<ZONE_ID>"
  }
}
```

### Why it is useful

- Agents can answer operational questions directly: top hosts, top paths, error spikes, firewall matches, bot traffic changes, Workers latency regressions.
- A wrapper can hide GraphQL complexity by shipping canned queries such as `cf analytics top-paths`, `cf analytics errors`, `cf analytics waf`, `cf analytics workers`.
- It also allows account-wide rollups and time-window comparisons that are ideal for incident triage and automated reporting.

### Why Wrangler does not already solve it well

Wrangler is a developer-platform CLI. Its documented command surface is centered on Workers and adjacent platform resources, not cross-product analytics querying. There is no general-purpose Wrangler analytics query workflow comparable to "run arbitrary GraphQL against Cloudflare analytics and normalize results".

### Likely required permissions

- Zone scope: `Analytics Read`
- Account scope: `Account Analytics Read`

### Example endpoints / request shapes

- `POST https://api.cloudflare.com/client/v4/graphql`
- Example use cases in docs include querying zone HTTP events, Workers metrics, and Access login events.

### Rough CLI ideas

- `cf analytics top-paths --zone example.com --since 1h`
- `cf analytics waf-hits --zone example.com --group-by rule --since 24h`
- `cf analytics workers-latency --account acct --script api-worker --since 6h`
- `cf analytics query --file query.graphql --var zoneTag=...`

### Sources

- GraphQL overview: <https://developers.cloudflare.com/analytics/graphql-api/>
- Query payload and endpoint: <https://developers.cloudflare.com/analytics/graphql-api/getting-started/execute-graphql-query/>
- Datasets: <https://developers.cloudflare.com/analytics/graphql-api/features/data-sets/>
- Wrangler commands: <https://developers.cloudflare.com/workers/wrangler/commands/>

## 2. Logpush job orchestration, validation, and health automation

### What raw Cloudflare API enables

Cloudflare exposes full CRUD for Logpush jobs, ownership challenges, destination validation, field discovery, option validation, and dataset inspection.

- `POST /zones/{zone_id}/logpush/ownership`
- `POST /zones/{zone_id}/logpush/validate/destination/exists`
- `GET /zones/{zone_id}/logpush/datasets/{dataset_id}/fields`
- `POST /zones/{zone_id}/logpush/jobs`
- Account-scoped equivalents exist for account datasets

### Why it is useful

- Log pipelines are high-value automation targets and painful to configure by hand.
- The API supports full preflight flows: validate destination, prove ownership, validate log fields, create job, then audit job health.
- A wrapper could make it practical for agents to stand up logs during incidents or bootstrap observability for new zones/accounts.

### Why Wrangler does not already solve it well

Wrangler does not document a Logpush management surface. This area sits outside its core Workers-oriented CLI remit.

### Likely required permissions

- Zone scope: `Logs Write`
- Some Zero Trust datasets additionally require `Zero Trust: PII Read` per Logpush permissions docs

### Example endpoints / request shapes

```json
POST /client/v4/zones/{zone_id}/logpush/ownership
{
  "destination_conf": "s3://my-bucket/logs/{DATE}?region=us-east-1&sse=AES256"
}
```

```json
POST /client/v4/zones/{zone_id}/logpush/jobs
{
  "name": "example.com http requests",
  "destination_conf": "s3://my-bucket/logs/{DATE}?region=us-east-1&sse=AES256",
  "dataset": "http_requests",
  "output_options": {
    "field_names": ["RayID", "ClientIP", "EdgeResponseStatus"],
    "timestamp_format": "rfc3339"
  }
}
```

### Rough CLI ideas

- `cf logs setup http-requests --zone example.com --to s3://bucket/logs/{DATE}?region=us-east-1`
- `cf logs fields --zone example.com --dataset http_requests`
- `cf logs validate --zone example.com --dataset http_requests --fields RayID,ClientIP`
- `cf logs health --account acct`

### Sources

- API configuration: <https://developers.cloudflare.com/logs/logpush/logpush-job/api-configuration/>
- Ownership challenge: <https://developers.cloudflare.com/logs/logpush/logpush-job/logpush-ownership-challenge/>
- Datasets and fields: <https://developers.cloudflare.com/logs/logpush/logpush-job/datasets/>
- Permissions: <https://developers.cloudflare.com/logs/logpush/permissions>

## 3. Rulesets-as-code across WAF, redirects, rate limiting, cache, and transforms

### What raw Cloudflare API enables

The Rulesets API manages phase entry points and custom rules across many Cloudflare products. A single API family can create or update WAF custom rules, rate limits, redirects, cache rules, managed ruleset deployments, and more.

- View phase entry point ruleset
- Add rules incrementally
- Replace or deploy rulesets
- Work at zone or account scope

### Why it is useful

- This is one of the strongest automation surfaces in Cloudflare because many operational controls converge on one API model.
- Agents could do structured diffing, dry runs, backups, promotion between environments, and targeted changes by hostname/path.
- It is particularly useful for incident response, migrations, and repeatable policy rollouts.

### Why Wrangler does not already solve it well

Wrangler does not expose a general rules-engine management experience. Even where specific products have dashboards or Terraform support, there is still a gap for ad hoc CLI automation across phases and products.

### Likely required permissions

Depending on phase and product:

- `Zone WAF Edit`
- `Account WAF Edit`
- `Transform Rules Edit`
- `Cache Rules Edit`
- `Single Redirect Edit`
- `Config Rules Edit`
- `Origin Rules Edit`

### Example endpoints / request shapes

- `GET /zones/{zone_id}/rulesets/phases/{phase}/entrypoint`
- `PUT /zones/{zone_id}/rulesets/{ruleset_id}`
- `POST /zones/{zone_id}/rulesets/{ruleset_id}/rules`

Example concept:

```json
POST /client/v4/zones/{zone_id}/rulesets/{ruleset_id}/rules
{
  "action": "block",
  "expression": "(http.host eq \"example.com\" and http.request.uri.path starts_with \"/admin\")",
  "description": "Block public admin path"
}
```

### Rough CLI ideas

- `cf rules export --zone example.com --phase http_request_firewall_custom`
- `cf rules add block --zone example.com --expr 'http.request.uri.path starts_with "/admin"'`
- `cf redirects sync --zone example.com redirects.csv`
- `cf waf managed override --zone example.com --tag wordpress --action log`

### Sources

- Rulesets API overview: <https://developers.cloudflare.com/ruleset-engine/rulesets-api/>
- Endpoints: <https://developers.cloudflare.com/ruleset-engine/rulesets-api/endpoints/>
- Phases: <https://developers.cloudflare.com/ruleset-engine/about/phases/>
- Phase API: <https://developers.cloudflare.com/api/resources/rulesets/subresources/phases>
- Redirect example: <https://developers.cloudflare.com/rules/url-forwarding/single-redirects/create-api/>

## 4. Zero Trust Access application, policy, and service-token automation

### What raw Cloudflare API enables

Cloudflare exposes API endpoints to create reusable Access policies, create service tokens, and manage broader Zero Trust configuration. This is a strong fit for non-human service access and internal app bootstrap workflows.

- `GET/POST /accounts/{account_id}/access/policies`
- `POST /accounts/{account_id}/access/service_tokens`
- Broader Zero Trust configuration APIs are supported and Cloudflare explicitly documents API/Terraform-first management

### Why it is useful

- Agents can bootstrap protected internal apps end-to-end: create policy, mint service token, wire CI secret outputs, and hand back exact headers to use.
- It supports short-lived or environment-specific auth patterns for tests, deploy previews, admin tooling, cron jobs, and machine-to-machine access.
- It pairs naturally with Tunnel, Hyperdrive, or private network access workflows.

### Why Wrangler does not already solve it well

Wrangler now covers Tunnel management, but it is not a full Zero Trust control-plane CLI. Access policies, reusable policies, service-token lifecycles, and policy testing are separate needs not addressed by the Wrangler command index.

### Likely required permissions

- `Access: Policies Edit`
- `Access: Apps and Policies Edit`
- `Access: Service Tokens Edit`
- Possibly `Zero Trust Write` depending on endpoint family

### Example endpoints / request shapes

```json
POST /client/v4/accounts/{account_id}/access/policies
{
  "name": "Allow CI to staging",
  "decision": "non_identity",
  "include": [
    {
      "service_token": {
        "token_id": "<TOKEN_ID>"
      }
    }
  ]
}
```

```json
POST /client/v4/accounts/{account_id}/access/service_tokens
{
  "name": "staging-ci"
}
```

### Rough CLI ideas

- `cf access app bootstrap --account acct --hostname admin.example.com`
- `cf access token create --account acct --name staging-ci`
- `cf access policy add service-auth --account acct --app app-id --token staging-ci`
- `cf access export-headers --account acct --token-id ...`

### Sources

- Zero Trust API/Terraform posture: <https://developers.cloudflare.com/cloudflare-one/api-terraform/>
- Reusable policy endpoints: <https://developers.cloudflare.com/api/resources/zero_trust/subresources/access/subresources/policies>
- Service tokens docs: <https://developers.cloudflare.com/cloudflare-one/access-controls/service-credentials/service-tokens>
- Service token create endpoint: <https://developers.cloudflare.com/api/resources/zero_trust/subresources/access/subresources/service_tokens/methods/create>

## 5. Account-owned API token factory and token hygiene automation

### What raw Cloudflare API enables

Cloudflare supports creating account-owned API tokens via API, including scoped policies, TTLs, and IP restrictions. This is fundamentally different from just "using a token"; it enables automated credential minting and delegation.

- `POST /accounts/{account_id}/tokens`
- `GET /accounts/{account_id}/tokens/permission_groups`
- `GET /accounts/{account_id}/tokens/verify`

### Why it is useful

- Agents can mint least-privilege service principals on demand for specific zones, accounts, and time windows.
- This is ideal for ephemeral deployment credentials, customer-specific delegation, one-off migrations, and automated break-glass workflows.
- A wrapper can standardize safe defaults: short TTL, IP filters, scoped permissions, and named templates.

### Why Wrangler does not already solve it well

Wrangler authenticates to Cloudflare, but it is not an API-token lifecycle and governance CLI. There is no documented Wrangler surface for token creation, scoping, inventory, hygiene checks, or templated delegation.

### Likely required permissions

- `API Tokens Write`
- `API Tokens Read`

### Example endpoints / request shapes

```json
POST /client/v4/accounts/{account_id}/tokens
{
  "name": "readonly token",
  "policies": [
    {
      "effect": "allow",
      "resources": {
        "com.cloudflare.api.account.zone.<ZONE_ID>": "*"
      },
      "permission_groups": [
        { "id": "<ZONE_READ_PERMISSION_ID>" },
        { "id": "<DNS_READ_PERMISSION_ID>" }
      ]
    }
  ],
  "not_before": "2026-06-21T00:00:00Z",
  "expires_on": "2026-06-22T00:00:00Z",
  "condition": {
    "request.ip": {
      "in": ["203.0.113.10/32"]
    }
  }
}
```

### Rough CLI ideas

- `cf token mint dns-read --zone example.com --ttl 2h --ip 203.0.113.10/32`
- `cf token mint analytics-read --account acct --zones prod-a,prod-b`
- `cf token audit --account acct`
- `cf token template zone-dns-write --zone example.com --expires 1h`

### Sources

- Create via API: <https://developers.cloudflare.com/fundamentals/api/how-to/create-via-api/>
- Account-owned tokens: <https://developers.cloudflare.com/fundamentals/api/get-started/account-owned-tokens>
- Token permissions: <https://developers.cloudflare.com/fundamentals/api/reference/permissions/>
- Token verify endpoint: <https://developers.cloudflare.com/api/resources/user/subresources/tokens/methods/verify>

## 6. DNS batch transactions, scans, and review workflows

### What raw Cloudflare API enables

Cloudflare's DNS API goes beyond single-record CRUD. It supports batch operations and zone scanning/review flows that can accelerate large migrations and automation-heavy DNS maintenance.

- `GET /zones/{zone_id}/dns_records`
- `POST /zones/{zone_id}/dns_records/batch`
- Trigger / list / review scanned DNS records

### Why it is useful

- Bulk cutovers, mail-provider migrations, subdomain fleet changes, and verification-record churn are common automation tasks.
- A wrapper can do plan/apply previews, detect conflicts, and group related changes into one batch request.
- DNS scan and review endpoints are useful when onboarding legacy domains and converting discovered records into a reviewed change set.

### Why Wrangler does not already solve it well

DNS is outside Wrangler's main scope. This is exactly the kind of general Cloudflare control-plane functionality where a purpose-built CLI adds value.

### Likely required permissions

- `DNS Read`
- `DNS Write`
- Often `Zone Read` for surrounding workflows

### Example endpoints / request shapes

```json
POST /client/v4/zones/{zone_id}/dns_records/batch
{
  "deletes": [{ "id": "<OLD_RECORD_ID>" }],
  "patches": [{ "id": "<RECORD_ID>", "ttl": 60, "proxied": false }],
  "posts": [
    {
      "type": "CNAME",
      "name": "app",
      "content": "target.example.net",
      "ttl": 60,
      "proxied": false
    }
  ]
}
```

### Rough CLI ideas

- `cf dns plan records.yaml`
- `cf dns apply records.yaml --batch`
- `cf dns scan trigger --zone example.com`
- `cf dns scan review --zone example.com`

### Sources

- List DNS records: <https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/list>
- Batch DNS records: <https://developers.cloudflare.com/api/resources/dns/subresources/records/methods/batch>
- DNS quick scan: <https://developers.cloudflare.com/dns/zone-setups/reference/dns-quick-scan>

## 7. Load balancing monitors, pools, previews, and health references

### What raw Cloudflare API enables

Cloudflare exposes APIs for load balancer pools, monitors, monitor groups, previews, references, and health details. This includes operations that are highly automatable and operationally sensitive.

- `GET/POST/PATCH /accounts/{account_id}/load_balancers/monitors`
- `POST /accounts/{account_id}/load_balancers/monitors/{monitor_id}/preview`
- `GET/POST/PATCH /accounts/{account_id}/load_balancers/pools`
- `POST /accounts/{account_id}/load_balancers/pools/{pool_id}/preview`
- `GET /accounts/{account_id}/load_balancers/pools/{pool_id}/references`

### Why it is useful

- Excellent fit for failover automation, blue/green pool changes, regional maintenance windows, and traffic-shift runbooks.
- Preview/reference endpoints are particularly valuable because they allow safer agent workflows before mutation.
- Monitor Groups add richer health logic that agents could templatize.

### Why Wrangler does not already solve it well

Load Balancing is not part of Wrangler's documented command surface. This is an operational networking API, not a developer-platform deployment primitive.

### Likely required permissions

- `Load Balancing: Monitors and Pools Edit`
- `Load Balancers Edit`
- Read variants for inspection-only commands

### Example endpoints / request shapes

- `POST /accounts/{account_id}/load_balancers/pools/{pool_id}/preview`
- `GET /accounts/{account_id}/load_balancers/pools/{pool_id}/references`

Example pool create shape:

```json
POST /client/v4/accounts/{account_id}/load_balancers/pools
{
  "name": "app-primary",
  "origins": [
    { "name": "iad-1", "address": "203.0.113.10", "enabled": true },
    { "name": "sfo-1", "address": "203.0.113.20", "enabled": true }
  ],
  "monitor": "<MONITOR_ID>"
}
```

### Rough CLI ideas

- `cf lb pool preview --account acct --pool app-primary`
- `cf lb drain origin --account acct --pool app-primary --origin iad-1`
- `cf lb attach-monitor-group --account acct --pool api --group critical-db`
- `cf lb references --account acct --pool api`

### Sources

- Pools: <https://developers.cloudflare.com/load-balancing/pools/>
- Monitors: <https://developers.cloudflare.com/load-balancing/monitors/>
- Monitor groups: <https://developers.cloudflare.com/load-balancing/monitors/monitor-groups/>
- API reference index excerpt: <https://developers.cloudflare.com/api>

## 8. Notification destinations and alert policy automation

### What raw Cloudflare API enables

Cloudflare exposes APIs for notification destinations and alert policies, including webhook destinations and available alert types.

- `GET /accounts/{account_id}/alerting/v3/destinations/eligible`
- `GET/POST /accounts/{account_id}/alerting/v3/destinations/webhooks`
- Alert types and policy CRUD endpoints under alerting APIs

### Why it is useful

- Agents can wire incident notifications as part of setup instead of leaving monitoring half-finished.
- This is useful when standing up Logpush, Tunnel, DDoS, billing, or SSL alerts alongside the primary configuration change.
- A wrapper can standardize webhook test flows, secrets, and policy templates.

### Why Wrangler does not already solve it well

Notifications are outside Wrangler's published scope. This is a general account-control-plane automation problem.

### Likely required permissions

- `Notifications Edit`
- Possibly additional product read permissions depending on alert types

### Example endpoints / request shapes

```json
POST /client/v4/accounts/{account_id}/alerting/v3/destinations/webhooks
{
  "name": "ops-webhook",
  "url": "https://hooks.example.com/cloudflare",
  "secret": "<shared-secret>"
}
```

### Rough CLI ideas

- `cf notify webhook create --account acct --name ops --url https://hooks.example.com/cloudflare`
- `cf notify policy enable tunnel-health --account acct --webhook ops`
- `cf notify alert-types --account acct`

### Sources

- Destinations API: <https://developers.cloudflare.com/api/resources/alerting/subresources/destinations>
- Alert types API: <https://developers.cloudflare.com/api/resources/alerting/subresources/available_alerts/methods/list>
- Webhook docs: <https://developers.cloudflare.com/notifications/get-started/configure-webhooks/>

## Excluded or lower-priority areas

- Wrangler-covered developer resources: D1, KV, R2, Queues, Hyperdrive, Vectorize, VPC, Pipelines, Workflows, Containers, Secrets Store.
- Basic Worker deployment and version control: clearly covered by Wrangler.
- Basic Tunnel creation and management: now present in Wrangler `tunnel` commands, so not a top gap by itself.
- Terraform-first infrastructure management: useful, but not the target here because the requirement is raw API opportunities for a custom CLI wrapper.

## Recommended shortlist

If building only a small number of new CLI surfaces, the best sequence is:

1. GraphQL analytics query runner
2. Logpush orchestration
3. Rulesets-as-code
4. Access/service-token automation
5. API token factory

That sequence gives the highest practical value for agents:

- observe
- export logs
- change request-handling behavior
- secure internal services
- mint least-privilege credentials for the above
