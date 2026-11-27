# Yevqo Pro REST API

Base URL: `https://app.yevqo.cc.cd/v1`

The versioned REST API is available to accounts with `users.status = 'pro'` and to the configured primary administrator account. API keys inherit the same workspace and project authorization as the account that created them.

## API keys

Create and manage keys in **Profile → Yevqo Pro REST API**.

- `GET /api/api-keys` — list active keys (session authentication).
- `POST /api/api-keys` with `{"name":"CI"}` — create a key (the plaintext is returned once).
- `DELETE /api/api-keys/{id}` — revoke a key.

Keys use the `yevqo_live_` prefix. Only a SHA-256 digest is stored; plaintext is never retrievable. Send keys only over HTTPS and revoke any key that may have leaked.

## Authentication

Every `/v1/*` request requires:

```http
Authorization: Bearer yevqo_live_<secret>
```

Example:

```bash
curl https://app.yevqo.cc.cd/v1/projects \
  -H "Authorization: Bearer $YEVQO_API_KEY"
```

## Endpoints

The v1 API currently exposes the existing authenticated Control Plane operations:

| Resource | Endpoints |
|---|---|
| Account | `GET /me` |
| Projects | `GET, POST /projects`; project detail, environments, members, repository and deployment subroutes |
| Devices | `GET /devices`; device detail and command subroutes |
| Applications | `GET, POST /sites`; application detail and lifecycle subroutes |
| Deployments | `GET /deployments`; project deployment operations |
| Databases | `/databases`, `/databases/servers`, `/database-connections` and detail subroutes |
| Backups | `/backups`, restore, transfer, chunk and schedule routes |
| Domains | `/domains` |
| Monitoring and logs | `/monitoring`, `/logs` |
| Workspace operations | `/workspaces`, `/notifications`, `/audit-logs`, `/reports`, `/scheduled-tasks`, `/templates` |

HTTP methods and request bodies follow the matching Dashboard API endpoint. Responses use the existing JSON response format. Unsupported routes return 404; invalid credentials return 401; account plan restrictions return 403.

## Security and limits

- API keys have full account-level API access; they do not bypass workspace roles, project permissions, resource ownership or server-side validation.
- Revoking a key takes effect on the next request.
- Do not place API keys in browser code, source control, public issue reports or logs.
- The primary configured administrator account is exempt from the Free connected-device cap. Pro accounts have an unlimited connected-device allowance. Other Free accounts remain limited to one connected device.
- Pro activation/billing is not automated by this API implementation; account plan assignment remains an administrative operation.
