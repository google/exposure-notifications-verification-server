# Fast Error Budget Alert

This alert fires when 2% of the error budget, as determined by the
availability SLO, is consumed in an hour.

Our SLO is defined in [slos.tf](../../../terraform/alerting/slos.tf).

The error budget start burning when 5xx error are returned.

## Triage steps

First thing first, communicate to the internal chat to raise awareness.

We currently have Error Burdget Burn SLO defined for the following services:
* `apiserver`
* `adminapi`
* `server`
* `enx-redirect`

Confirm which service is firing the alert by looking at the alert name. It should be in the format of `FastErrorBudgetBurn-SERVICE_NAME` or `SlowErrorBudgetBurn-SERVICE_NAME`.

Next, on Cloud Console you can click on the Navigation Menu -> `Monitoring` -> `Metrics Explorer`. 
Select `Query Editor` and  use the following Metrics Query Language (MQL) queries to see the 5xx errors:

```text
fetch cloud_run_revision
| metric 'run.googleapis.com/request_count'
| filter (metric.response_code_class == '5xx')
| align rate(1m)
| every 1m
```

You have confirmed the system is having 500 errors, now you can go check the logs. 
To do so you have to click on Navigation Menu -> `Logging` -> `Logs Explorer`
Use the following log filter to determine what went wrong:

```text
resource.type="cloud_run_revision"
resource.labels.service_name="${SERVICE_NAME}"
severity=ERROR
```
You will find relevant error messages under the `jsonPayload` field. Assess the message to analyze what component of the system could be failing. 

If you see no relevant logs, the root cause could be the load balancer. To view the load balancer logs:

```text
resource.type="http_load_balancer"
httpRequest.status >= 500
```
