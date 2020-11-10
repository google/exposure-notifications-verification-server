# API access

Access to the verification server API requires an API key. An API key typically
corresponds to an individual mobile application or individual human. There are
two types of API keys:

-   `DEVICE` - Intended for a mobile application to call the `cmd/apiserver` to
    perform the two step protocol to exchange verification _codes_ for
    verification _tokens_, and verification _tokens_ for verification
    _certificates_.

-   `ADMIN` - Intended for public health authority internal applications to
    integrate with this server. **We strongly advise putting additional
    protections in place such as an external proxy authentication.**


# API usage

The following APIs exist for the API server (`cmd/apiserver`). All APIs are JSON
over HTTP. You should always specify the `content-type` and `accept` headers as
`application/json`. Check with your server operator for the specific hostname.

## Authenticating

All endpoints require an API key passed via the `X-API-Key` header. The server
supports HTTP/2, so the header key is case-insensitive. For example:

```sh
curl https://example.encv.org/api/method \
  --header "content-type: application/json" \
  --header "accept: application/json" \
  --header "x-api-key: abcd.5.dkanddssk"
```

API keys will _generally_ be in a particular format, but developers should not
attempt to build any intelligence on this format. The format, length, and
character set are not guaranteed to remain the same between releases.

## Error reporting

All errors contain an English language error message and well defines `ErrorCode`.
The `ErrorCodes` are defined in [api.go](https://github.com/google/exposure-notifications-verification-server/blob/main/pkg/api/api.go).

# API Methods

## `/api/verify`

Exchange a verification code for a long term verification token.

**VerifyCodeRequest**

```json
{
  "code": "<the code>",
  "accept": ["confirmed"],
  "padding": "<bytes>"
}
```

* `accept` is an _optional_ list of the diagnosis types that the client is willing to process. Accepted values are
  * `["confirmed"]`
  * `["confirmed", "likely"]`
  * `["confirmed", "likely", "negative"]`
  * It is not possible to get just `likely` or just `negative` - if a client
        passes `likely` they are indiciating they can process both `confirmed` and `likely`.
* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.

**VerifyCodeResponse**

```json
{
  "testtype": "<test type string>",
  "symptomDate": "YYYY-MM-DD",
  "testDate": "YYYY-MM-DD",
  "token": "<JWT verification token>",
  "error": "",
  "errorCode": "",
  "padding": "<bytes>"
}
```

* `symptomDate` and `testDate` will be present of that information was
  provided when the verification code was generated. These fields are
  omitted in the response body if corresponding date was not set.
* `padding` is a field that obfuscates the size of the response body to a
  network observer. The server _may_ generate and insert a random number of
  base64-encoded bytes into this field. The client should not process the
  padding.

Possible error code responses. New error codes may be added in future releases.

| ErrorCode               | HTTP Status | Retry | Meaning |
|-------------------------|-------------|-------|---------|
| `unparsable_request`    | 400         | No    | Client sent an request the sever cannot parse |
| `code_invalid`          | 400         | No    | Code invalid or used, user may need to obtain a new code. |
| `code_expired`          | 400         | No    | Code has expired, user may need to obtain a new code. |
| `code_not_found`        | 400         | No    | The server has no record of that code. |
| `invalid_test_type`     | 400         | No    | The client sent an accept of an unrecgonized test type |
| `missing_date`          | 400         | No    | The realm requires either a test or symptom date, but none was provided. |
| `unsupported_test_type` | 412         | No    | The code may be valid, but represents a test type the client cannot process. User may need to upgrade software. |
|                         | 500         | Yes   | Internal processing error, may be successful on retry. |

## `/api/certificate`

Exchange a verification token for a verification certificate (for sending to a key server)

**VerificationCertificateRequest**

```json
{
  "token": "token from verifyCodeResponse",
  "ekeyhmac": "hmac of exposure keys, base64 encoded",
  "padding": "<bytes>"
}
```

* `token`: must be exactly the string that was returned on the `/api/verify` request
* `ekeyhmac`: must be calculated on the client
  * The client generates an HMAC secret and calcualtes the HMAC of the actual TEK data
  * [Plaintext generation algorithm](https://github.com/google/exposure-notifications-server/blob/main/docs/design/verification_protocol.md)
  * [Sample HMAC generation (Go)](https://github.com/google/exposure-notifications-server/blob/main/pkg/verification/utils.go)
  * The key server will re-calculate this HMAC and it MUST match what is presented here.
* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.


**VerificationCertificateResponse**

 ```json
{
  "certificate": "<JWT verification certificate>",
  "error": "",
  "errorCode": "",
  "padding": "<bytes>"
}
```

* `padding` is a field that obfuscates the size of the response body to a
  network observer. The server _may_ generate and insert a random number of
  base64-encoded bytes into this field. The client should not process the
  padding.

Possible error code responses. New error codes may be added in future releases.

| ErrorCode               | HTTP Status | Retry | Meaning |
|-------------------------|-------------|-------|---------|
| `token_invalid`         | 400         | No    | The provided token is invalid, or already used to generate a certificate |
| `token_expired`         | 400         | No    | Code invalid or used, user may need to obtain a new code. |
| `hmac_invalid`          | 400         | No    | The `ekeyhmac` field, when base64 decoded is not the right size (32 bytes) |
|                         | 500         | Yes   | Internal processing error, may be successful on retry. |

# Admin APIs

These APIs are available on the admin server and require and `ADMIN` level API key.

## `/api/issue`

Request a verification code to be issued. Accepts [optional] symptom date and test dates in ISO 8601 format. These can be in local time, if a timezone offset is provided. If a phone number is provided and the realm is configured with SMS credentials, then an SMS will be dispatched according to the realm's settings.

**IssueCodeRequest**

```json
{
  "symptomDate": "YYYY-MM-DD",
  "testDate": "YYYY-MM-DD",
  "testType": "<valid test type>",
  "tzOffset": 0,
  "phone": "+CC Phone number",
  "padding": "<bytes>"
}
```

* `sypmtomDate` and `testDate` are both optional.
  * only one will be encoded into the eventually issued certificate
  * symptom date is always preferred to test date
* `testType`
  * Must be `confirmed`, `likely`, `negative`
  * valid values depends on your realm's settings
* `tzOffset`
  * Offset in minutes of the user's timezone. Positive, negative, 0, or omitted (using the default of 0) are all valid. 0 is considered to be UTC.
* `phone`
  * Phone number to send the SMS too
* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.

**IssueCodeResponse**

```json
{
  "uuid": "string UUID",
  "code": "short verification code",
  "expiresAt": "RFC1123 formatted string timestamp",
  "expiresAtTimestamp": 0,
  "expiresAt": "RFC1123 UTC timestamp",
  "expiresAtTimestamp": 0,
  "longExpiresAt": "RFC1123 UTC timestamp",
  "longExpiresAtTimestamp": 0,
  "error": "descriptive error message",
  "errorCode": "well defined error code from api.go",
}
```

* `uuid`
  * UUID is a handle which allows the issuer to track status of the issued verification code.
* `code`
  * The OTP code which may be exchanged by the user for a signing token.
* `expiresAt`
  * RFC1123 formatted string timestamp, in UTC.
	After this time the code will no longer be accepted and is eligible for deletion.
* `expiresAtTimestamp`
  * represents Unix, seconds since the epoch. Still UTC.
	After this time the code will no longer be accepted and is eligible for deletion.
* `longExpiresAt`
  * represents the time that the link containing a 'long' verification code expires (if one was issued)
* `longExpiresAtTimestamp`
  * Unix, seconds since the epoch for `longExpiresAt`
* `padding` is a field that obfuscates the size of the response body to a
  network observer. The server _may_ generate and insert a random number of
  base64-encoded bytes into this field. The client should not process the
  padding.

## `/api/checkcodestatus`

Checks the status of a previous issued code, looking up by UUID.

**CheckCodeStatusRequest**

```json
{
  "uuid": "UUID for code to check",
  "padding": "<bytes>"
}
```

* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.

**CheckCodeStatusResponse**

```json
{
  "claimed": false,
  "expiresAtTimestamp": 0,
  "longExpiresAtTimestamp": 0,
  "error": "descriptive error message",
  "errorCode": "well defined error code from api.go",
  "padding": "<bytes>"
}
```

* `claimed`
  * boolean indicating if the code was used or not
* `expiresAtTimestamp`
  * seconds since the epoch indicating expiry time in UTC
* `longExpiresAtTimestamp`
  * seconds since the epoch for the SMS link expiry time in UTC
* `padding` is a field that obfuscates the size of the response body to a
  network observer. The server _may_ generate and insert a random number of
  base64-encoded bytes into this field. The client should not process the
  padding.

## `/api/expirecode`

Expires an unclaimed code. IF the code has been claimed an error is returned.

**ExpireCodeRequest**

```json
{
  "uuid": "UUID of the code to expire",
  "expiresAtTimestamp": 0,
  "longExpiresAtTimestamp": 0,
  "error": "descriptive error message",
  "errorCode": "well defined error code from api.go",
  "padding": "<bytes>"
}
```

* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.

The timestamps are updated to the new expiration time (which will be in the
past).

# Chaffing requests

In addition to "real" requests, the server also accepts chaff (fake) requests.
These can be used to obfuscate real traffic from a network observer or server
operator. To initiate a chaff request, set the `X-Chaff` header on your request:

```sh
curl https://example.encv.org/api/endpoint \
  --header "content-type: application/json" \
  --header "accept: application/json" \
  --header "x-chaff: 1"
```

The client should still send a real request with a real request body (the body
will not be processed). The server will respond with a fake response that your
client **MUST NOT** process or parse. The response will not be a valid JSON 
object.

Client's should sporadically issue chaff requests to mirror real-world usage.

# Response codes overview

You can expect the following responses from this API:

-   `400` - The client made a bad/invalid request. Search the JSON response body
    for the `"errors"` key. The body may be empty.

-   `401` - The client is unauthorized. This could be an invalid API key or
    revoked permissions. This usually has no `"errors"` key, but clients can try
    to read the JSON body to see if there's additional information (it may be
    empty)

-   `404` - The client made a request to an invalid URL (routing error). Do not
    retry.

-   `405` - The client used the wrong HTTP verb. Do not retry.

-   `412` - The client requested a precondition that cannot be satisfied.

-   `429` - The client is rate limited. Check the `X-Retry-After` header to
    determine when to retry the request. Clients can also monitor the
    `X-RateLimit-Remaining` header that's returned with all responses to
    determine their rate limit and rate limit expiration.

-   `5xx` - Internal server error. Clients should retry with a reasonable
    backoff algorithm and maximum cap.
