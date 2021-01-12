# CloudSchedulerJobFailed

Cloud Scheduler is responsible for running the following jobs:
  * `appsync-worker` - Syncs the published list of mobile apps to the server's database.
  * `backupdatabase-worker` - Generates a database backup every interval. Currently configured to every 6 hours.
  * `cleanup-worker` - Runs the database cleanup. Currently configured to run every hour.
  * `e2e-default-workflow` - Runs the [End to End test](../../../../cmd/e2e-runner/main.go) every 0,10,20,30,40,50,55 of the hour.
  * `e2e-revise-workflow` - Runs the same end to end test every 0,5,15,25,35,45,55 of the hour. It uses a different flag to avoid race conditions.
  * `modeler-worker` - Implements periodic statistical calculations. Currently configured to run once a day.

If Cloud Scheduler produces any ERROR level logs when running any of the above jobs, the system will send an alert notification. It indicates the job is failing for some reason.

## Triage Steps

Go to Logs Explorer, use the following filter:

```
resource.type="cloud_scheduler_job"
severity=ERROR
```

See what the error message is. Depending on the error, you may also need
to check the corresponding Cloud Run service's log.
