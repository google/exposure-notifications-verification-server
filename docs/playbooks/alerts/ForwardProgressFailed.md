# ForwardProgressFailed

This alert fires when background jobs have not made forward progress in an acceptable amount of time. The alert will include the name of the job that is failing to make forward progress. The jobs are invoked in the background.

- `appsync-worker` - Syncs the published list of mobile apps to the server's database.

- `backup-worker` - Generates a backups every interval.

- `cleanup-worker` - Performs a variety of cleanup tasks including purging old data, secrets, and keys.

- `e2e-default` - Runs the [End to End test](../../../../cmd/e2e-runner/main.go).

- `e2e-redirect` - Runs the End to End workflow using the `enx-redirect` service.

- `e2e-revise` - Runs the same end to end test to the revise endpoint.

- `emailer-anomalies` - Generates and sends emails to realm contacts if the code claim ratio drops below the historical average.

- `emailer-sms-errors` - Generates and sends emails to realm contacts if the number of SMS errors in the current UTC day exceeds a provided threshold.

- `modeler-worker` - Implements periodic statistical calculations.

- `realm-key-rotation-worker` - Rotates realm signing keys.

- `rotation-worker` - Rotates system signing keys (primarily for tokens).

- `stats-puller-worker` - Imports statistics from the key server.

Each job runs on a different interval. Check your Terraform configuration to see how frequently a specific job runs.

## Triage Steps

When one of the jobs does not return success within a configured interval, this alert will fire. For most cases, this means the job has already failed 2+ times.

To begin triage, locate the logs for the corresponding service name using the Logs Explorer:

```text
resource.type="cloud_run_revision"
resource.labels.service_name="<service>"
```

For example, if the failing service was `appsync`:

```text
resource.type="cloud_run_revision"
resource.labels.service_name="appsync"
```

Check for errors in the logs.

### Service-specific triage steps

- `emailer-anomalies` - The most likely reason this job is failing is because one of the email addresses provided by a realm admin is invalid or inaccessible. The logs will show the invalid email address(es). You can remove the invalid email address(es) or ask the other realm administrators to update their configuration.

- `emailer-sms-errors` - Same as `emailer-anomalies`.
