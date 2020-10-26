# Elevated 5xx Alert

This alert fires when greater than 2 requests per second returning status code of 500 -> 599 for greater than five minutes.

* First check if the site is up
   * Check if https://encv.org/ loads (or appropriate domain for this environment)
   * Check if you can login
   * If you can, admins aren't affected
   * If you can't login everyone is affected
* Post in chat that you've got the alert
   * Communicate to your team that you are actively looking at this alert to lower confusion.
* Look at the metrics dashboard
   * Load https://console.cloud.google.com/monitoring/dashboards
   * Open the dashboard titled "Verification Server", and look at the top left graph
   * Or query the Metrics explorer with the following MQL
```
cloud_run_revision::run.googleapis.com/request_count
| align rate()
| every 1m
| [resource.service_name, metric.response_code_class]
```
   * Look for servers with elevated 5xx
* Look at request logs, you can navigate by hand or use the following query
```
resource.type="cloud_run_revision"
resource.labels.service_name="e2e-runner"
severity=ERROR
```
