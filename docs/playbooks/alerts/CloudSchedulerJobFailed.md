# CloudSchedulerJobFailed

This is currently the main alert to track errors overall in the system. You should be familiar with each cron job scheduled below to easily identify the error message when this alert is issued.

Cloud Scheduler is responsible for running the following jobs:
  * `appsync-worker` - Syncs the published list of mobile apps to the server's database.
  * `backupdatabase-worker` - Generates a database backup every interval. Currently configured to every 6 hours.
  * `cleanup-worker` - Runs the database cleanup. Currently configured to run every hour.
  * `e2e-default-workflow` - Runs the [End to End test](../../../../cmd/e2e-runner/main.go) every 10 minutes.
  * `e2e-enx-redirect-workflow` - Runs the End to End workflow using the `enx-redirect` service every 10 minutes.
  * `e2e-revise-workflow` - Runs the same end to end test to the revise endpoint every 10 minutes.
  * `modeler-worker` - Implements periodic statistical calculations. Currently configured to run once a day.
  * `realm-key-rotation-worker` - Job that implements periodic secret rotation. Scheduled to run twice an hour for realm certificates.
  * `rotation-worker` -  Implements periodic secret rotation that runs every 5 minutes for tokens. 
  * `stats-puller-worker` -  Pulls statistics from the key server every 30 minutes.

If Cloud Scheduler produces any ERROR level logs when running any of the above jobs, the system will send an alert notification. It indicates the job is failing for some reason.

## Triage Steps

Go to Logs Explorer, use the following filter:

```
resource.type="cloud_scheduler_job"
severity=ERROR
```

See what the error message is. Depending on the error, you may also need
to check the corresponding Cloud Run service's log.
