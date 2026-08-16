<picture>
  <source media="(prefers-color-scheme: dark)" srcset=".github/norn-wordmark-dark.svg">
  <img src=".github/norn-wordmark-light.svg" alt="Norn" height="26">
</picture>

# An issue tracker you run yourself

Open source and keyboard-first. Use the managed EU cloud, or deploy it on your own hardware.

[norn.so](https://norn.so) · [Documentation](https://docs.norn.so) · [Changelog](https://docs.norn.so/changelog) · [Roadmap](https://roadmap.norn.so) · [Status](https://status.norn.so)

**Yours to run.** Deploy it yourself, or sign up for the managed EU cloud. Same software, same features, either way.

**No per-seat pricing.** Members and issues are unlimited and never counted. Cloud plans meter storage and agent usage.

**SSO on every tier.** SAML and OIDC cost nothing, self-hosted included. Audit logs and directory sync are the paid additions.

**Built for the keyboard.** Every action has a shortcut.

## The tracker

Boards, grouped and filtered issue lists, cycles, and triage. Drag between states, or change them from the keyboard. Issues carry a description, sub-issues, relations, and full history. Cycles show scope, progress, and what rolls over. Triage collects everything unsorted in one queue.

Search, navigate and act from the same place:

| | |
| --- | --- |
| `⌘ K` | Search |
| `C` | New issue |
| `J` `K` | Move the cursor |
| `1` – `6` | Set status |

## MCP and agents

Authorize once for every workspace, scoped to your own permissions and revocable at any time. Agents act under their own name in issue history.

Read the [MCP docs](https://docs.norn.so/api).

## SSO

SAML and OIDC on every tier, self-hosted included. Audit logs and directory sync are the paid additions. See the [SSO docs](https://docs.norn.so/sso).

## Importing

Importers for Jira, Linear, GitHub Issues and CSV. Original references stay resolvable.

## Why Norn

Issue trackers hold the record of what a team is doing and why. That record is worth keeping under your own control, on infrastructure you choose, without paying by the head for the privilege.

Norn is open source under AGPL-3.0. Run it on your own hardware, or let us run it in the EU.

## Reasons to switch

| | Norn | Linear | Jira | Plane |
| --- | --- | --- | --- | --- |
| Self-host | Yes | No | No for new customers | Yes |
| Open source | AGPL-3.0 | No | No | AGPL-3.0 core |
| SAML and OIDC | Every tier | Enterprise | Guard add-on | Commercial edition |
| Per-seat pricing | No | Yes | Yes | Yes |
| Member limit on free | None | None | 10 users | 12 seats |
| Issue limit | None | 250 on free | None | None |
| EU-only hosting | Yes | EU or US region | Region options | Cloud regions |
| SCIM | Paid | Enterprise | Guard add-on | Enterprise Grid |
| MCP across workspaces | One authorization | Per workspace | Per site | Per workspace |
| Full export | Free | Yes | Yes | Yes |

Checked 14 August 2026.

## Run it

Sign up for the [managed EU cloud](https://app.norn.so/sign-up), or run it on infrastructure you control — start with the [self-hosting docs](https://docs.norn.so/self-hosting). The official [Helm chart](https://docs.norn.so/self-hosting/helm) installs the Norn API, web dashboard, worker, database migrations, and authorisation policy seed; by default it also installs single-node PostgreSQL, Valkey, and Garage.

## Development

`make env` writes the local environment, then `docker compose up -d` starts the development dependencies: PostgreSQL, Valkey, Garage, Mailpit, and Keycloak. `make test` and `make lint` check the tree.

## License

[AGPL-3.0](LICENSE)
