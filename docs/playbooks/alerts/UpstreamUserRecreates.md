# UpstreamUserRecreates

This alert fires when a significant portion of requests result in re-creating
users in the upstream auth provider.

## Triage Steps

-   Verify the upstream auth provider status is online and configured correctly.

-   Inspect logs to see if requests to create users are failing.
