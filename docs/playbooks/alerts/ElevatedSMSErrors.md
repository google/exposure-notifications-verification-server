# Elevated SMS Errors Alert

This alert fires when one or more realms have an increased number of errors returned from the Twilio API.

**This alert can have false positives**. There could be legitimate reasons why
the percentage of codes claimed decreases by more than one standard deviation.


## Triage Steps

Verify Twilio is currently operational by inspecting their status dashboard at
https://status.twilio.com. This system only uses the **Programmable Messaging**
service, so you should only inspect the status for that service.

-   If there's a widespread outage, it likely affects all realms. Depending on
    the duration of the outage, you may want to send proactive communication to
    realm administrators. Usually these issues resolve in minutes, but sometimes
    they can take hours to resolves.

-   If there is not a widespread outage, this could signal an error with the
    realm's SMS configuration or phone number (spam filtering). Check the Twilio
    console for errors. In particular, the error `E30007` indicates
    carrier-level filtering.

In the event the phone number or message is being filtered as spam, immediately
file a ticket with Twilio and communicate with the PHA to stop issuing codes
until the issue resolves. Note, sometimes this can take up to 72 hours to
resolve.
