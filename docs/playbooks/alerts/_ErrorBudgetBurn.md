# Error Budget Burn

This playbook applies to both
[FastErrorBudgetBurn](./FastErrorBudgetBurn.md) and
[SlowErrorBudgetBurn](./SlowErrorBudgetBurn.md).

Our SLO is defined in [slos.tf](../../../terraform/alerting/slos.tf).

The error budget start burning when 5xx error are returned.

## Triage steps

First thing first, communicate to the internal chat to raise awareness.

Use the following MQL queries to see which service is returning 5xx
errors:

```
fetch cloud_run_revision
| metric 'run.googleapis.com/request_count'
| filter (metric.response_code_class == '5xx')
| align rate(1m)
| every 1m
```

(NB: we currently only have SLO defined for `apiserver` service so this
step may not be necessary)

While at this, you can also add a "group by" to check whether the error
is correlated to a new release.

Use the following log filter to determine what went wrong:

```
resource.type="cloud_run_revision"
resource.labels.service_name="${SERVICE_NAME}"
severity=ERROR
```
