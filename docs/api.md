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


## API usage

The following APIs exist for the API server (`cmd/apiserver`). All APIs are JSON
over HTTP. You should always specify the `content-type` and `accept` headers as
`application/json`.

### Authenticating

All endpoints require an API key passed via the `X-API-Key` header. The server
supports HTTP/2, so the header key is case-insensitive. For example:

```sh
curl https://example.encv.org/api/endpoint \
  --header "content-type: application/json" \
  --header "accept: application/json" \
  --header "x-api-key: abcd.5.dkanddssk"
```

API keys will _generally_ be in a particular format, but developers should not
attempt to build any intelligence on this format. The format, length, and
character set are not guaranteed to remain the same between releases.

### Endpoints

-   `/api/verify` - Exchange a verification code for a long term verification
    token.

    **VerifyCodeRequest:**

    ```json
    {
      "code": "<the code>"
    }
    ```

    **VerifyCodeResponse:**

    ```json
    {
      "TestType": "<test type string>",
      "SymptomDate": "YYYY-MM-DD",
      "VerificationToken": "<JWT verification token>",
      "Error": ""
    }
    ```

-   `/api/certificate` - Exchange a verification token for a verification
    certificate (for key server)

    **VerificationCertificateRequest:**

    ```json
    {
      "VerificationToken": "token from verifyCodeResponse",
      "ekeyhmac": "hmac of exposure keys"
    }
    ```

    **VerificationCertificateResponse:**

    ```json
    {
      "Certificate": "<JWT verification certificate>",
      "Error": ""
    }
    ```

### Chaffing requests

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
client **MUST NOT** process. Client's should sporadically issue chaff requests.

### Response codes

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

-   `429` - The client is rate limited. Check the `X-Retry-After` header to
    determine when to retry the request. Clients can also monitor the
    `X-RateLimit-Remaining` header that's returned with all responses to
    determine their rate limit and rate limit expiration.

-   `5xx` - Internal server error. Clients should retry with a reasonable
    backoff algorithm and maximum cap.
