# CodesClaimedRatioAnomaly

This alert fires when a realm's predictive modeling for the ratio of `codes
claimed / codes issue` (the percentage of codes claimed) drops by more than one
standard deviation below the rolling 14 day average.

**This alert can have false positives**. There could be legitimate reasons why
the percentage of codes claimed decreases by more than one standard deviation.


## Triage Steps

1.  Identify the realm. The metric will be tagged with the realm ID. You can map
    this ID back to the realm in the system admin console of the verification
    server UI.

1.  Join the realm if you are not already a member.

1.  Inspect the realm stats. Sometimes this alert can fire if there was a public
    holiday or a long weekend. Use your judgement on whether the drop in codes
    claimed is of concern.

1.  Check the status page for the cloud hosting provider to see if there are any
    known issues.

1.  If you determine the drop to be of concern, notify the realm admin. This
    process varies by server operator, so use your internal playbooks for
    identifying and alerting realm admins. Inform the realm admin that you are
    investigating the incident. Ask if they have additional information or
    insights.

1.  Manually issue a code and check that it is properly delivered without error.

1.  Check the server logs for the `server` and `adminapi` components to see if
    there are errors related to code issuing.

1.  If the realm is issuing codes via a third-party SMS provider, check the
    third-party SMS provider's logs to see if the messages are being rejected as
    spam.

    If messages are being rejected, filed a high-priority ticket with the SMS
    provider.
