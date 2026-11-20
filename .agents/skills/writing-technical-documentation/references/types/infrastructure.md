# Infrastructure and operations docs

Contents: resources and their purpose, naming conventions, environments, deployment pipeline, access model, networking, secrets handling (names only), backups and recovery objectives (RPO and RTO), scaling limits and quotas, cost drivers, monitoring and alerts with thresholds, and disaster recovery steps.

Read the IaC as the source of truth, and read `references/reading-order.md` section 6 for AWS. Use placeholders for account IDs, ARNs, and hostnames unless the user wants real ones. Summarize the IAM permissions that matter for operating the system without pasting policies. Do not state an RPO, RTO, or SLA unless an owner confirms it.

```markdown
# <System> infrastructure

Last verified: <YYYY-MM-DD> against <commit>.

## Environments

| Environment | Purpose | How it differs |
|---|---|---|
| <name> | <> | <config differences> |

## Resources

| Resource | Purpose | Defined in |
|---|---|---|
| <name pattern> | <> | `<path>` |

## Deploy and roll back

<Pipeline, approvals, rollback steps.>

## Access and secrets

<Roles that matter. Secret names and where they live, never values.>

## Limits and cost drivers

| Limit | Value | What happens at the limit |
|---|---|---|
| <quota> | <number> | <failure mode> |

## Monitoring

| Alert | Threshold | Runbook |
|---|---|---|
| <name> | <value> | [<runbook>](<path>) |

## Recovery

<Backups, recovery steps, and objectives an owner has confirmed.>
```
