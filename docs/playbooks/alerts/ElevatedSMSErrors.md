# Elevated SMS Errors Alert

This alert fires when your realm has an increased number of errors returned from
the Twilio API. Specifically, if there are more than 50 errors in a 24 hour
period.

**This alert can have false positives**. There could be legitimate reasons why
SMS messages might be failing, such as invalid phone numbers, which do not
indicate an issue in the system.


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


## Tuning

You may find that particular error codes are the source of false positives. It
may be helpful to _exclude_ these particular error codes from the alerting
threshold. To do this, contact your server operator with the codes to omit from
alerting.
