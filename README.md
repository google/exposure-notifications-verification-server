What does this change do to CI?

# Exposure Notifications Verification Server

This is a reference implementation for an Exposure Notifications [verification server](https://developers.google.com/android/exposure-notifications/verification-system), part of the broader [Google Exposure Notifications system](https://github.com/google/exposure-notifications-server).


## About the Server

This server follows the [high level flow](https://developers.google.com/android/exposure-notifications/verification-system#flow-overview)
for a verification system:

1.  Authenticates and authorizes humans using [Identity
    Platform](https://cloud.google.com/identity-platform).

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

## More resources

-   [Architecture](docs/architecture.md)
-   [API guide](docs/api.md)
-   [Exposure Notifications Express SMS / Deep Link Handling](docs/link_handling.md)
-   [Realm administration guide](docs/realm-admin-guide.md)
-   [Case worker guide](docs/case-worker-guide.md)
-   [System admin guide](docs/system-admin-guide.md)
-   [Development](docs/development.md)
-   [Using the Cloud SQL Proxy](docs/using-cloud-sql-proxy.md)
-   [Production](docs/production.md)
-   [Testing](docs/testing.md)
