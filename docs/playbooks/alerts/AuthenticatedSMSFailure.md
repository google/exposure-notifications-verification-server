# AuthenticatedSMSFailure

This alert fires when a realm with Authenticated SMS enabled has an increase in
the number of SMS signature failures. This metric is incremented regardless of
whether the system is configured to fail open or fail closed on failed attempts
to sign the SMS.

## Triage Steps

Go to Logs Explorer, use the following filter:

```
resource.type="cloud_run_revision"
jsonPayload.logger="issueapi.sendSMS"
jsonPayload.message="failed to sign sms"
```

The most common cause for this error is upstream key unavailability. If the
upstream key management system is unavailable, the system will fail to sign
SMS messages. In most cases, the error self-resolves.

## Mitigation

In the event the error does not self-resolve after a time, you may want to
configure the system to fail open (continue on error) for authenticated SMS
failures. Note: the default behavior is to fail open, so unless you have changed
the default behavior, no action is required.

To re-configure the system to fail open, set `SMS_FAIL_CLOSED=false` on the
`adminapi` and `server` components in the environment and restart the services.
