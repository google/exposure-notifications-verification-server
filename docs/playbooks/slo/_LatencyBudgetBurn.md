# Elevated Latency Greater than 2s

This alert fires when any
`loadbalancing.googleapis.com/https/backend_latencies stream` is above a
threshold of 2s for greater than 5 minutes.

Note this is the latency between the load balancer and the Cloud Run
service and therefore includes the latency in between.

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

Additional queries

There are some additional metrics you may want to check to understand
where the latencies come from.

Go to Metric Explorer (Google Cloud Console hamburger menu -> Monitoring
-> Metrics explorer, choose the right workspace, click Query Editor) to
use these queries:

```
# 99 percentile database client latency
generic_task::custom.googleapis.com/opencensus/go.sql/client/latency
| align delta(1m)
| every 1m
| [metric.go_sql_method, metric.go_sql_status, metric.go_sql_error],
  [val: percentile(value.latency, 99)]
```

```
# 99 percentile grpc client latency
generic_task::custom.googleapis.com/opencensus/grpc.io/client/roundtrip_latency
| align delta(1m)
| every 1m
| [metric.grpc_client_method], [val: percentile(value.roundtrip_latency, 99)]
```

```
# 99 percentile redis latency
generic_task::custom.googleapis.com/opencensus/redis/client/roundtrip_latency
| align delta(1m)
| every 1m
| group_by [metric.cmd],
    [value_roundtrip_latency_percentile:
       percentile(value.roundtrip_latency, 99)]
```
