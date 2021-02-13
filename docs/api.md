<!-- TOC depthFrom:1 -->autoauto- [API access](#api-access)auto- [API usage](#api-usage)auto    - [Authenticating](#authenticating)auto    - [Error reporting](#error-reporting)auto- [API Methods](#api-methods)auto    - [`/api/verify`](#apiverify)auto    - [`/api/certificate`](#apicertificate)auto- [Admin APIs](#admin-apis)auto    - [`/api/issue`](#apiissue)auto        - [Client provided UUID to prevent duplicate SMS](#client-provided-uuid-to-prevent-duplicate-sms)auto    - [`/api/batch-issue`](#apibatch-issue)auto        - [Handling batch partial success/failure](#handling-batch-partial-successfailure)auto    - [`/api/checkcodestatus`](#apicheckcodestatus)auto    - [`/api/expirecode`](#apiexpirecode)auto    - [`/api/stats/*`](#apistats)auto- [Chaffing requests](#chaffing-requests)auto- [Response codes overview](#response-codes-overview)autoauto<!-- /TOC -->

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

-   `STATS` - Intended for public health authorities to gather automated
    statistics.


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
        passes `likely` they are indicating they can process both `confirmed` and `likely`.
* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.

**VerifyCodeResponse**

```json
http 200
{
  "testtype": "<test type string>",
  "symptomDate": "YYYY-MM-DD",
  "testDate": "YYYY-MM-DD",
  "token": "<JWT verification token>",
  "padding": "<bytes>"
}

or

{
  "error": "",
  "errorCode": "",
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

| ErrorCode             | HTTP Status | Retry | Meaning                                                                                      |
| --------------------- | ----------- | ----- | -------------------------------------------------------------------------------------------- |
| `unparsable_request`  | 400         | No    | Client sent an request the sever cannot parse                                                |
| `code_invalid`        | 400         | No    | Code invalid or used, user may need to obtain a new code.                                    |
| `code_expired`        | 400         | No    | Code has expired, user may need to obtain a new code.                                        |
| `code_not_found`      | 400         | No    | The server has no record of that code.                                                       |
| `invalid_test_type`   | 400         | No    | The client sent an accept of an unrecognized test type                                       |
| `maintenance_mode   ` | 429         | Yes   | The server is temporarily down for maintenance. Wait and retry later.                        |
| `quota_exceeded`      | 429         | Yes   | The realm has run out of its daily quota allocation for issuing codes. Wait and retry later. |
|                       | 500         | Yes   | Internal processing error, may be successful on retry.                                       |

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
  * The client generates an HMAC secret and calculates the HMAC of the actual TEK data
  * [Plaintext generation algorithm](https://github.com/google/exposure-notifications-server/blob/main/docs/design/verification_protocol.md)
  * [Sample HMAC generation (Go)](https://github.com/google/exposure-notifications-server/blob/main/pkg/verification/utils.go)
  * The key server will re-calculate this HMAC and it MUST match what is presented here.
* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.


**VerificationCertificateResponse**

```json
http 200
{
  "certificate": "<JWT verification certificate>",
  "padding": "<bytes>"
}

or

{
  "error": "",
  "errorCode": "",
}
```

* `padding` is a field that obfuscates the size of the response body to a
  network observer. The server _may_ generate and insert a random number of
  base64-encoded bytes into this field. The client should not process the
  padding.

Possible error code responses. New error codes may be added in future releases.

| ErrorCode             | HTTP Status | Retry | Meaning                                                                    |
| --------------------- | ----------- | ----- | -------------------------------------------------------------------------- |
| `token_invalid`       | 400         | No    | The provided token is invalid, or already used to generate a certificate   |
| `token_expired`       | 400         | No    | Code invalid or used, user may need to obtain a new code.                  |
| `hmac_invalid`        | 400         | No    | The `ekeyhmac` field, when base64 decoded is not the right size (32 bytes) |
| `maintenance_mode   ` | 429         | Yes   | The server is temporarily down for maintenance. Wait and retry later.      |
|                       | 500         | Yes   | Internal processing error, may be successful on retry.                     |

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
  "smsTemplateLabel": "my sms template",
  "padding": "<bytes>",
  "uuid": "optional string UUID",
  "externalIssuerID": "external-ID",
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
  * Phone number to send the SMS to. If a phone number is provided, but the SMS text
    message fails to send, the API will return a 4xx client error.
* `smsTemplateLabel`
  * If the realm has more than one SMS template defined, this may be optionally specify
    the label of the message template which the server should compose. If omitted, the
    default template will be used.
* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.
* `uuid` is optional as request input. The server will generate a uuid on response if omitted.
  * This is a handle which allows the issuer to track status of the issued verification code.
* `externalIssuerID` is an optional field supplied by the API caller to uniquely
  identify the entity making this request. This is useful where callers are
  using a single API key behind an ERP, or when callers are using the
  verification server as an API with a different authentication system. This
  field is optional.

  * The information provided is stored exactly as-is. If the identifier is
    uniquely identifying PII (such as an email address, employee ID, SSN, etc),
    the caller should apply a cryptographic hash before sending that data. **The
    system does not sanitize or encrypt these external IDs, it is the caller's
    responsibility to do so.**

**IssueCodeResponse**

```json
http 200
{
  "uuid": "string UUID",
  "code": "short verification code",
  "expiresAt": "RFC1123 formatted string timestamp",
  "expiresAtTimestamp": 0,
  "expiresAt": "RFC1123 UTC timestamp",
  "expiresAtTimestamp": 0,
  "longExpiresAt": "RFC1123 UTC timestamp",
  "longExpiresAtTimestamp": 0,
}

or

{
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

Possible error code responses. New error codes may be added in future releases.

| ErrorCode               | HTTP Status | Retry | Meaning                                                                                                         |
| ----------------------- | ----------- | ----- | --------------------------------------------------------------------------------------------------------------- |
| `unparsable_request`    | 400         | No    | Client sent an request the sever cannot parse                                                                   |
| `invalid_test_type`     | 400         | No    | The client sent an accept of an unrecognized test type                                                          |
| `missing_date`          | 400         | No    | The realm requires either a test or symptom date, but none was provided.                                        |
| `invalid_date`          | 400         | No    | The provided test or symptom date, was older or newer than the realm allows.                                    |
| `invalid_test_type`     | 400         | No    | The test type is not a valid test type (a string that is unknown to the server).                                |
| `uuid_already_exists`   | 409         | No    | The UUID has already been used for an issued code                                                               |
| `maintenance_mode   `   | 429         | Yes   | The server is temporarily down for maintenance. Wait and retry later.                                           |
| `quota_exceeded`        | 429         | Yes   | The realm has run out of its daily quota allocation for issuing codes. Wait and retry later.                    |
| `unsupported_test_type` | 412         | No    | The code may be valid, but represents a test type the client cannot process. User may need to upgrade software. |
|                         | 500         | Yes   | Internal processing error, may be successful on retry.                                                          |

### Client provided UUID to prevent duplicate SMS

Every response includes `uuid` to track the status of an issued code. Optionally `IssueCodeRequest` may also take in a `uuid` from the client. This can be useful when implementing retry logic to ensure the same request does not send more than one SMS to the same patient. Once successful, subsequent requests with the same `uuid` will give status `409` `uuid_already_exists`.

The `uuid` field is stored on the server for tracking. It is therefore important that this field remains meaningless to the server. The client may generate them randomly and store them locally, or use a one-way hash of `phone` using a locally known HMAC key.

This may also be used as an external handle to coordinate among multiple external issuers. For example, a testing lab which issues codes might attach a `uuid` to case information before handing off data to the state or other agencies to prevent multiple notifications to the patient.

## `/api/batch-issue`

Request a batch of verification codes to be issued. Accepts a list of IssueCodeRequest. See [`/api/issue`](#apiissue) for details of the fields of a single issue request and response. The indices of the respective
`codes` arrays will match each request/response pair unless a server error occurs which results in an empty `codes`
array response.

This API currently supports a limit of up 10 codes per request.

### Handling batch partial success/failure
This API is *not atomic* and does not follow the [typical guidelines for a batch API](https://google.aip.dev/233) due to the sending of SMS
messages.

The server attempts to issue every code in the batch. If errors are encountered, each item in `codes` will contain the error details for
the corresponding code. The overall request will get the error status code of the first seen error, although some codes may have
succeeded.

eg.
```
{
  codes: [
    {
      "error": "the first code failed",
      "errorCode": "missing_date",
    },
    {
      "uuid": "string UUID",
      "code": "short verification code",
      "expiresAt": "RFC1123 formatted string timestamp",
      "expiresAtTimestamp": 0,
      "expiresAt": "RFC1123 UTC timestamp",
      "expiresAtTimestamp": 0,
    },
    {
      "error": "the third code failed",
      "errorCode": "unparsable_request",
    },
  ],
  error: "the first code failed",
  errorCode: "missing_date",
}
```

**BatchIssueCodeRequest**

```json
{
  "codes" : [
    {
      "symptomDate": "YYYY-MM-DD",
      "testDate": "YYYY-MM-DD",
      "testType": "<valid test type>",
      "tzOffset": 0,
      "phone": "+CC Phone number",
      "uuid": "optional string UUID",
      "externalIssuerID": "external-ID",
    },
    {
      ...
    },
  ],
  "padding": "<bytes>"
}
```

**BatchIssueCodeResponse**

Note: The `error` and `errorCode` of the outer response body will match the first error from the `codes` array.
The response http code will also match the first seen error. The caller should iterate `codes` to handle errors
for each code response. The index of each responses will match the index of the original request.

```
{
  "codes": [
    {
      "uuid": "string UUID",
      "code": "short verification code",
      "expiresAt": "RFC1123 formatted string timestamp",
      "expiresAtTimestamp": 0,
      "expiresAt": "RFC1123 UTC timestamp",
      "expiresAtTimestamp": 0,
      "longExpiresAt": "RFC1123 UTC timestamp",
      "longExpiresAtTimestamp": 0,
      "error": "[optional] descriptive error message",
      "errorCode": "[optional] well defined error code from api.go",
    },
    {
      ...
    },
  ],
  "padding": "<bytes>"
  "error": "[optional] descriptive error message. The first seen error from 'codes'",
  "errorCode": "[optional] well defined error code from api.go. The first-seen errorCode of 'codes'",
}
```

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
http 200
{
  "claimed": false,
  "expiresAtTimestamp": 0,
  "longExpiresAtTimestamp": 0,
  "padding": "<bytes>"
}

or

{
  "error": "descriptive error message",
  "errorCode": "well defined error code from api.go",
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

Expires an unclaimed code. If the code has been claimed an error is returned.

**ExpireCodeRequest**

```json
http 200
{
  "uuid": "UUID of the code to expire",
  "expiresAtTimestamp": 0,
  "longExpiresAtTimestamp": 0,
  "padding": "<bytes>"
}

or

{
  "error": "descriptive error message",
  "errorCode": "well defined error code from api.go",
}
```

* `padding` is a _recommended_ field that obfuscates the size of the request
  body to a network observer. The client should generate and insert a random
  number of base64-encoded bytes into this field. The server does not process
  the padding.

The timestamps are updated to the new expiration time (which will be in the
past).


## `/api/stats/*`

The statistics APIs are forward-compatible. That means no fields will be
_removed_ or _changed_ without prior notice, but the API may _add_ new fields or
endpoints without notice.

-   `/api/stats/realm.{csv,json}` - Daily statistics for the realm, including
    codes issued, codes claimed, tokens claimed, and invalid attempts.

-  `/api/stats/realm/key-server.{csv,json}` - Daily statistics gathered from the
   key-server if enabled for the realm. This includes publish requests, EN days
   active before upload, and onset-to-upload distribution.

-   `/api/stats/realm/users.{csv,json}` - Daily statistics for codes issued by
    realm user. These statistics only include codes issued by humans logged into
    the verification system.

-   `/api/stats/realm/users/:id.{csv,json}` - Daily statistics for codes issued
    by the user with the given ID. These statistics only include codes issued by
    that human user logged into the verification system for the currently
    authorized realm.

-   `/api/stats/realm/api-keys/:id.{csv,json}` - Daily statistics for the API
    key with the given ID. For _admin_ API keys, the statistics will include
    codes issued. For _device_ API keys, the statistics will include codes
    claimed and codes invalid.

-   `/api/stats/realm/external-issuers.{csv,json}` - Daily statistics for codes
    issued by external issuers. These statistics only include codes issued by
    the API where an `externalIssuer` field was provided.


# Chaffing requests

In addition to "real" requests, the server also accepts chaff (fake) requests.
These can be used to obfuscate real traffic from a network observer or server
operator.

Chaff requests:

* MUST send the `X-API-Key` header with a valid API key (otherwise you will
  get an unauthorized error)
* MUST be sent via a `POST` request, otherwise you will get an invalid method
  error
* SHOULD send a valid JSON body with random padding similar in size as the rest
  of the client requests so that chaff requests appear the same on the wire as
  other valid requests.
* SHOULD send chaff requests for both `/verify`, `/certificate`, and key server
  publish endpoints.

To initiate a chaff request, set the `X-Chaff` header on your request:

```sh
curl https://apiserver.example.com/api/verify \
  --header "x-api-key: YOUR-API-KEY" \
  --header "content-type: application/json" \
  --header "accept: application/json" \
  --header "x-chaff: 1" \
  --request POST \
  --data '{"padding":"base64 encoded padding"}'
```

The client should still send a real request with a real request body (the body
will not be processed). The server will respond with a fake response that your
client **MUST NOT** process or parse. The response will not be a valid JSON
object.

Client's should sporadically issue chaff requests to mirror real-world usage
for both the `/verify`, `/certificate`, and key server publish endpoints.

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

-   `429` - The client is rate limited. Check the `Retry-After` header to
    determine when to retry the request. Clients can also monitor the
    `X-RateLimit-Remaining` header that's returned with all responses to
    determine their rate limit and rate limit expiration.

-   `5xx` - Internal server error. Clients should retry with a reasonable
    backoff algorithm and maximum cap.
