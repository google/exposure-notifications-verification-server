# Exposure Notifications Verification System | Reference Server


As part of the broader [Google Exposure Notification](https://github.com/google/exposure-notifications-server)
reference server efforts, this repository contains the reference implementation
for a [verification server](https://developers.google.com/android/exposure-notifications/verification-system).

## About the Server

Following the [high level flow](https://developers.google.com/android/exposure-notifications/verification-system#flow-overview)
for the verification system, this server:

1. Handles human user authorization using [Firebase Authentication](https://firebase.google.com/docs/auth)
2. Provides a Web interface for a case investigation epidemiologist (epi) to
   enter test parameters (status + test date) and issue a _verification code_
	 * Verification codes are __8 numeric digits__ so that they can be easily
	   read over a phone call or send via SMS text message.
	 * Verification codes are valid for a short duration (1 hour)
3. Provides a JSON-over-HTTP API for exchanging the verification code for
   a _verification token_.
	 * Verification tokens are signed [JWTs](htts://jwt.io) that are valid for
	   24 hours (configurable)
4. Provides a JSON-over-HTTP API for exchanging the verification token for a
   _verification certificate_. This API call also requires an [HMAC](https://en.wikipedia.org/wiki/HMAC)
	 of the Temporary Exposure Key (TEK) data+metatata. This HMAC value is
	 signed by the verification server to be later accepted by an exposure
	 notifications server. This same TEK data used to generate the HMAC here, must
	 be passed to the exposure notifications server, otherwise the request will
	 be rejected.
	 * Please see the documentation for the [HMAC Calculation](https://developers.google.com/android/exposure-notifications/verification-system#hmac-calc)
	 * The Verification Certificate is also a JWT

![Verification Flow](https://developers.google.com/android/exposure-notifications/images/verification-certificate-hmac-flow-diagram.png)

Architecture details

* 4 services combine to make this application. All servers are intended to be
  deployed in an autoscaled serverless environment.
  * `cmd/server` - Web UI for creating verification codes
  * `cmd/apiserver` - Server for mobile device applications to do verification
  * `cmd/adminapi` - Server for connecting existing PHA applications to the
     verification system. [Optional component]
  * `cmd/cleanup` - Server for cleaning up old data. Required in order to
    recycle and reuse verification codes over a longer period of time.
* PostgreSQL database for shared state
  * This codebase utilizes [GORM](https://gorm.io/), so it is possible to
	  easily switch to another supported SQL database.
* Redis cache used for distributed rate limiting.
* Relies on Firebase Authentication for handling of identity / login
 * As is, this project is configured to use username/password based login, but
   can easily be configured to use any firebase supported identity provider.

## Configuring your Development Environment for Running Locally

```shell
gcloud auth login && gcloud auth application-default login
```

Create a key ring and two signing keys

```shell
gcloud kms keyrings create --location=us signing
gcloud kms keys create token-signing --location=us --keyring=signing --purpose=asymmetric-signing --default-algorithm=ec-sign-p256-sha256
gcloud kms keys create certificate-signing --location=us --keyring=signing --purpose=asymmetric-signing --default-algorithm=ec-sign-p256-sha256
```

To get the resource name(s)

```shell
gcloud kms keys describe token-signing --keyring=signing --location=us
gcloud kms keys describe certificate-signing --keyring=signing --location=us
```

Finish setup and run the server.

```shell
gcloud auth login && gcloud auth application-default login

# In case you have this set, unset it to rely on gcloud.
unset GOOGLE_APPLICATION_CREDENTIALS

# Initialize Dev Settings
eval $(./scripts/dev init)
./scripts/dev dbstart

# Configure These settings to your firebase application
export FIREBASE_API_KEY="YOUR API KEY"
export FIREBASE_PROJECT_ID="YOUR-PROJECT-123456"
export FIREBASE_MESSAGE_SENDER_ID="789123456"
export FIREBASE_APP_ID="1:123456:web:abcd1234"
export FIREBASE_MEASUREMENT_ID="G-J12345C"

export FIREBASE_AUTH_DOMAIN="${FIREBASE_PROJECT_ID}.firebaseapp.com"
export FIREBASE_DATABASE_URL="https://${FIREBASE_PROJECT_ID}.firebaseio.com"
export FIREBASE_STORAGE_BUCKET="${FIREBASE_PROJECT_ID}.appspot.com"

export TOKEN_SIGNING_KEY="<Token Key Resource ID from Above>"
export CERTIFICATE_SIGNING_KEY="<Certificate Key Resource ID from Above>"


# D/L SA from Firebase https://console.firebase.google.com/project/project-name-123456/settings/serviceaccounts/adminsdk
export GOOGLE_APPLICATION_CREDENTIALS=/Users/USERNAME/Documents/project-name-123456-firebase-adminsdk-ab3-4cde56f78g.json

# Configure CSRF_AUTH_KEY. This is a 32 byte string base64 encoded.
export CSRF_AUTH_KEY=aGVsbG9oZWxsb2hlbGxvaGVsbG9oZWxsb2hlbGxvaGk=
export DEV_MODE=1

# Migrate DB
./scripts/dev dbmigrate

# create a user for whatever email address you want to use
go run ./cmd/add-users --email YOUR-NAME@DOMAIN.com --name "First Last" --admin true

go run ./cmd/server
```

## API Access

Access to the [APIs](https://github.com/google/exposure-notifications-verification-server/blob/main/pkg/api/api.go)
is controlled through API keys. Users with Admin access are able to issue API
keys and API keys have one of two levels of access: `DEVICE` or `ADMIN`.

* `DEVICE` - Intended for a mobile application to call the `cmd/apiserver` to
  to the two step verification protocol to exchange verification codes for
  verification tokens, and verification tokens for verification certificates.

* `ADMIN` - Intended for public health authority internal applications to
  integrate with this server. _Additional protection is recommended, i.e.
  service mesh or external authentication._

### API Guide for App Developers

For application developers, there are three APIs that are homed in the
`cmd/apiserver` that need to be utilized. All APIs are JSON over HTTP, only use
`POST`, and require that the API key be passed in the HTTP header `X-API-Key`.
The APIs are as follows.

1. `/api/verify` - Exchange a verification code for a long term verification
  token.
  * VerifyCodeRequest:
  ```json
  {
    "code": "<the code>"
  }
  ```
  * VerifyCodeResponse:
  ```json
  {
    "TestType": "<test type string>",
    "SymptomDate": "YYYY-MM-DD",
    "VerificationToken": "<JWT verification token>",
    "Error": ""
  }
  ```
2. `/api/certificate` - Exchange a verification token for a verification certificate (for key server)
  * VerificationCertificateRequest:
  ```json
  {
    "VerificationToken": "token from verifyCodeResponse",
    "ekeyhmac": "hmac of exposure keys"
  }
  ```
  * VerificationCertificateResponse:
  ```json
  {
    "Certificate": "<JWT verification certificate>",
    "Error": ""
  }
  ```
3. `/api/cover` - Send request from the device to the PHA server to "cover" the traffic. All devices in a region should connect to the
server multiple times per day to simulate token exchange.
  * CoverRequest:
  ```json
  {
    "Data": "<random string data>"
  }
  ```
  * CoverResponse:
  ```json
  {
    "Data": "random base64 encoded data",
    "Error": ""
  }
  ```


### Test Utilities

Using an Admin API key, one can request verification codes using
`cmd/get-token`.

```shell
go run ./cmd/get-code --type="confirmed" --onset="2020-07-14" --apikey="<ADMIN API KEY>"
```

From there, there are two tools that can be used to complete the code->token->certificate
exchange. To exchange the verification code for the verification token.

```shell
go run ./cmd/get-token --apikey="<DEVICE API KEY>" --code="<verificationCode>"
```

And to exchange the token for a verification certificate.

```shell
go run ./cmd/get-certificate --apikey="<DEVICE API KEY>" --token="<TOKEN FROM ABOVE>" --hmac="<HMAC TEKs to Certify>"
```

A complete end to end example:

```shell
exposure-notifications-verification-server on 🌱 readme [$!] via 🐹 v1.14.2
❯ go run ./cmd/get-code --apikey="BXlIlWxg3zgwDRPIVKF9QVshUbibOHI4cVsmXzxtJVx5FsBsr4/BNVSqzdaHXhEyAGf0X+xRp3rah9qipPB2kg" --type="likely" --onset="2020-07-10"
2020/07/16 13:56:51 Sending: {TestType:likely SymptomDate:2020-07-10}
2020/07/16 13:56:51 Result:
{VerificationCode:14404755 ExpiresAt:Thu, 16 Jul 2020 14:56:51 PDT Error:}

exposure-notifications-verification-server on 🌱 readme [$!] via 🐹 v1.14.2
❯ go run ./cmd/get-token --apikey="i9UhDG3kYj3eW0CslXMXujPJfbzJ0mJlLDN8zdFYiiDR6hrOrTm0UFSE6JSbW5qb9Af3/B+U+3nkmIxeopoMXA" --code="14404755"
2020/07/16 13:57:40 Sending: {VerificationCode:14404755}
2020/07/16 13:57:41 Result:
{TestType:likely SymptomDate:2020-07-10 VerificationToken:eyJhbGciOiJFUzI1NiIsImtpZCI6InYxIiwidHlwIjoiSldUIn0.eyJhdWQiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJleHAiOjE1OTUwMTk0NjEsImp0aSI6Im5BbVdJKzVnZDRuSG0wcnJiOGRGWUVwUExDdFpaK2dMOXZ5YjVCcDJIdmVHTndmeHV5ZS9rU2x2Q2NhSGovWEwrelh5K1U1L3JpdFh1SGt1eGtvc3dLam13ZlJ0ZUpRQWpqeEdYazV5cFpPeENySGM2Z1ZVZTdxdVVNZFVkRkpBIiwiaWF0IjoxNTk0OTMzMDYxLCJpc3MiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJzdWIiOiJsaWtlbHkuMjAyMC0wNy0xMCJ9.mxMsCwRUc6AtHNNjf_xjlxT4xJrwK2b1OkOvyWDmSKxJunaOBO_j9s4SCG_b3TbZn2eAPeqG8zNSu_YUzS5GYw Error:}

exposure-notifications-verification-server on 🌱 readme [$!] via 🐹 v1.14.2
❯ go run ./cmd/get-certificate --apikey="i9UhDG3kYj3eW0CslXMXujPJfbzJ0mJlLDN8zdFYiiDR6hrOrTm0UFSE6JSbW5qb9Af3/B+U+3nkmIxeopoMXA" --token="eyJhbGciOiJFUzI1NiIsImtpZCI6InYxIiwidHlwIjoiSldUIn0.eyJhdWQiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJleHAiOjE1OTUwMTk0NjEsImp0aSI6Im5BbVdJKzVnZDRuSG0wcnJiOGRGWUVwUExDdFpaK2dMOXZ5YjVCcDJIdmVHTndmeHV5ZS9rU2x2Q2NhSGovWEwrelh5K1U1L3JpdFh1SGt1eGtvc3dLam13ZlJ0ZUpRQWpqeEdYazV5cFpPeENySGM2Z1ZVZTdxdVVNZFVkRkpBIiwiaWF0IjoxNTk0OTMzMDYxLCJpc3MiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJzdWIiOiJsaWtlbHkuMjAyMC0wNy0xMCJ9.mxMsCwRUc6AtHNNjf_xjlxT4xJrwK2b1OkOvyWDmSKxJunaOBO_j9s4SCG_b3TbZn2eAPeqG8zNSu_YUzS5GYw" --hmac="2u1nHt5WWurJytFLF3xitNzM99oNrad2y4YGOL53AeY="
2020/07/16 13:59:24 Sending: {VerificationToken:eyJhbGciOiJFUzI1NiIsImtpZCI6InYxIiwidHlwIjoiSldUIn0.eyJhdWQiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJleHAiOjE1OTUwMTk0NjEsImp0aSI6Im5BbVdJKzVnZDRuSG0wcnJiOGRGWUVwUExDdFpaK2dMOXZ5YjVCcDJIdmVHTndmeHV5ZS9rU2x2Q2NhSGovWEwrelh5K1U1L3JpdFh1SGt1eGtvc3dLam13ZlJ0ZUpRQWpqeEdYazV5cFpPeENySGM2Z1ZVZTdxdVVNZFVkRkpBIiwiaWF0IjoxNTk0OTMzMDYxLCJpc3MiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJzdWIiOiJsaWtlbHkuMjAyMC0wNy0xMCJ9.mxMsCwRUc6AtHNNjf_xjlxT4xJrwK2b1OkOvyWDmSKxJunaOBO_j9s4SCG_b3TbZn2eAPeqG8zNSu_YUzS5GYw ExposureKeyHMAC:2u1nHt5WWurJytFLF3xitNzM99oNrad2y4YGOL53AeY=}
2020/07/16 13:59:24 Result:
{Certificate:eyJhbGciOiJFUzI1NiIsImtpZCI6InYxIiwidHlwIjoiSldUIn0.eyJyZXBvcnRUeXBlIjoibGlrZWx5Iiwic3ltcHRvbU9uc2V0SW50ZXJ2YWwiOjI2NTcyMzIsInRyaXNrIjpbXSwidGVrbWFjIjoiMnUxbkh0NVdXdXJKeXRGTEYzeGl0TnpNOTlvTnJhZDJ5NFlHT0w1M0FlWT0iLCJhdWQiOiJleHBvc3VyZS1ub3RpZmljYXRpb25zLXNlcnZlciIsImV4cCI6MTU5NDkzNDA2NCwiaWF0IjoxNTk0OTMzMTY0LCJpc3MiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJuYmYiOjE1OTQ5MzMxNjN9.gmIzjVUNLtmGHCEybx7NXw8NjTCKDBszUHeE3hnY9u15HISjtjpH2zE_5ZXk2nlRQT9OFQnIkogO8Bz4zLbf_A Error:}
```


## A Walkthrough of the Service
![Login](./docs/images/getting-started/0_login.png)
![Create Users](./docs/images/getting-started/1_create_user.png)
![Issue Verification Code](./docs/images/getting-started/2_issue_verification_code.png)
![Verification Code Issued](./docs/images/getting-started/3_verification_code_issued.png)
