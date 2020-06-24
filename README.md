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

Architecture details

* Single server (located in `cmd/server`) that provides all functionality
  * The server is stateless and suitable for autoscaled serverless container
	  environments.
* PostgreSQL database for shared state
  * This codebase utilizes [GORM](https://gorm.io/), so it is possible to
	  easily switch to another supported SQL database.
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

## Other Tools

From the UI, you can issue and `API KEY` for making API requests.

There are two tools for testing the end to end flow.

1. `go run ./cmd/get-token` to exchange the verification code for a verification
token.
2. `go run ./cmd/get-certificate` to exchange the verification token for a
verification certificate.

## A Walkthrough of the Service
![Login](./docs/images/getting-started/0_login.png)
![Create Users](./docs/images/getting-started/1_create_user.png)
![Issue Verification Code](./docs/images/getting-started/2_issue_verification_code.png)
![Verification Code Issued](./docs/images/getting-started/3_verification_code_issued.png)
