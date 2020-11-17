# Elevated Latency Greater than 2s

This alert fires when any loadbalancing.googleapis.com/https/backend_latencies stream is above a threshold of 2s for greater than 5 minutes.

Check:

```
fetch https_lb_rule
| metric 'loadbalancing.googleapis.com/https/backend_latencies'
| filter
    (resource.backend_name != 'NO_BACKEND_SELECTED'
     && resource.forwarding_rule_name == 'verification-server-https')
| align delta(1m)
| every 1m
| group_by [resource.backend_target_name],
    [value_backend_latencies_percentile:
       percentile(value.backend_latencies, 99)]
| condition val() > 2000 'ms'
```

Troubleshooting:

* Check which backend is having problems
* Check Logs Explorer for requests to this server on

```
resource.type="cloud_run_revision"
resource.labels.configuration_name="BACKEND NAME"
severity=INFO
(httpRequest.latency>"TIMEms" OR
jsonPayload.latency>"TIMEms")
```

* Check with the team if there are any ongoing changes recently
* Check with Google Cloud Support Status Dashboard for ongoing incidents
  * Open a support case.
