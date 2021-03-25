# ForwardProgressFailed

This alert fires when background jobs have not made forward progress in an acceptable amount of time. The alert will include the name of the job that is failing to make forward progress. The jobs are invoked in the background.

- `appsync-worker` - Syncs the published list of mobile apps to the server's database.

- `backup-worker` - Generates a backups every interval.

- `cleanup-worker` - Performs a variety of cleanup tasks including purging old data, secrets, and keys.

- `e2e-default` - Runs the [End to End test](../../../../cmd/e2e-runner/main.go).

- `e2e-redirect` - Runs the End to End workflow using the `enx-redirect` service.

- `e2e-revise` - Runs the same end to end test to the revise endpoint.

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
