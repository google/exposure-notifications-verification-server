# Fast Error Budget Alert

This alert fires when 2% of the error budget, as determined by the availability SLO, is consumed in an hour.

* First check if the site is up
   * Check if https://encv.org/ loads (or appropriate domain for this environment)
   * Check if you can login
   * If you can, admins aren't affected
   * If you can't login everyone is affected
* Post in chat that you've got the alert
   * Communicate to your team that you are actively looking at this alert to lower confusion.
* Look at services dashboard
   * Load https://console.cloud.google.com/monitoring/services
   * Look at the Verification Server service and determine its health
   * Look for servers with elevated 5xx
   * Look at request logs, you can navigate by hand or use the following query

```
resource.type="cloud_run_revision"
resource.labels.service_name="e2e-runner"
severity=ERROR
```