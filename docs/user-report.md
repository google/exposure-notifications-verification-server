
# Design for User Initiated Reporting

<!-- TOC depthFrom:2 depthTo:2 -->

- [User Report Functionality](#user-report-functionality)
- [Integration methods](#integration-methods)
- [Nonce protocol](#nonce-protocol)
- [Phone Number De-duplication](#phone-number-de-duplication)
- [Quotas / Modeling](#quotas--modeling)

<!-- /TOC -->

## User Report Functionality

The Exposure Notifications Verification Server provides the ability for
users of your application to request verification codes that will
cause their Temporary Exposure Keys (TEKs) to be shared as a `SELF_REPORT`
type.

## Integration methods

* API: for native application integration
* Web-view: must be launched from our application

### API

See the [API Documentation](api.md#apiuser-report) for the user report API.
This API requires a `DEVICE` API key and a `nonce` (see below).

### Web View

The Web view must be launched with a POST request with a `DEVICE` API key
and `nonce` (see below) with specific headers. See
[this document](user-report-webvidew.md) for information on launching the
Web view.

## Nonce protocol

When requesting a code, the device must generate `256` bytes of random data
and pass it to the verification server, base64 encoded.

This nonce value must be retained for 24 hours and sent on the next call
to the `api/verify` API by the device.

## Phone Number De-duplication

The verification server generates an HMAC of the phone number during
a user report request. This is used to ensure that a given phone
number does not initiate a user report more often than once
ever 90 days.

If a user request goes unclaimed, the phone number will be taken
off the de-duplication list within 90 minutes.

## Quotas / Modeling

User report codes count against rate limiting and anti-abuse quotas,
just like health authority issued codes.
