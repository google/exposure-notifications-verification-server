# Exposure Notifications Verification Server

This is a reference implementation for an Exposure Notifications [verification server](https://developers.google.com/android/exposure-notifications/verification-system), part of the broader [Google Exposure Notifications system](https://github.com/google/exposure-notifications-server).


## About the Server

This server follows the [high level flow](https://developers.google.com/android/exposure-notifications/verification-system#flow-overview)
for a verification system:

1.  Authenticates and authorizes humans using [Firebase
    authentication](https://firebase.google.com/docs/auth).

1.  Provides a web interface for epidemiologists (epi) to enter test parameters
    (e.g. status + test date) to issue a _verification code_.

    -   Short verification codes are typically 6-10 numeric digits and can be
        read over the phone to a patient. They expire quickly, usually in less
        than one hour.

    -   Longer verification codes can be sent directly to the patient via SMS.
        These codes generally last longer, like 24 hours.

1.  Provides a JSON-over-HTTP API for exchanging the verification _code_ for a
    verification _token_. This API is called by the patient's device.

    -   Verification tokens are signed [JWTs](htts://jwt.io) with a configurable
        validity period.

1.  Provides a JSON-over-HTTP API for exchanging the verification _token_ for a
    verification _certificate_. This API call also requires an
    [HMAC](https://en.wikipedia.org/wiki/HMAC) of the Temporary Exposure Key
    (TEK) data+metatata. This HMAC value is signed by the verification server to
    be later accepted by an exposure notifications server. This same TEK data
    used to generate the HMAC here, must be passed to the exposure notifications
    server, otherwise the request will be rejected.

    -   Please see the documentation for the [HMAC
        Calculation](https://developers.google.com/android/exposure-notifications/verification-system#hmac-calc)

	  -   The Verification Certificate is also a JWT


### Architecture diagram

![Verification Flow](https://developers.google.com/android/exposure-notifications/images/verification-flow.svg)

### Architecture details

-   This application is comprised of the following services which are designed
    to be serverless and scale independently:

    -   `cmd/server` - Web UI for creating verification codes

    -   `cmd/apiserver` - Server for mobile device applications to do
        verification

    -   `cmd/adminapi` - (optional) Server for connecting existing PHA
        applications to the verification system.

    -   `cmd/cleanup` - Server for cleaning up old data. Required in order to
        recycle and reuse verification codes over a longer period of time.

-   PostgreSQL database for shared state. Other databases may work, but we only
    aim to support Postgres at this time.

-   Redis for caching and distributed rate limiting.

-   Firebase authentication for login.


## More resources

-   [API Guide](docs/api.md)
-   [Realm admin guide](docs/realm_guide.md)
-   [User guide](docs/user_guide.md)
-   [Development](docs/development.md)
-   [Using the Cloud SQL Proxy](docs/using-cloud-sql-proxy.md)
-   [Production](docs/production.md)
-   [Testing](docs/testing.md)
