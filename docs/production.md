# Production

This page includes helpful tips for configuring things in production.

<!-- TOC depthFrom:2 depthTo:2 -->

- [Key management](#key-management)
- [Observability (tracing and metrics)](#observability-tracing-and-metrics)
- [User administration](#user-administration)
- [Rotating secrets](#rotating-secrets)
- [SMS with Twilio](#sms-with-twilio)
- [Identity Platform setup](#identity-platform-setup)
- [End-to-end test runner](#end-to-end-test-runner)
- [Architecture](#architecture)

<!-- /TOC -->

## Key management

The default production key management solution is [Google Cloud KMS][gcp-kms].
If you are using the Terraform configurations, the system will automatically
bootstrap and create the key rings and keys in Cloud KMS. If you are not using
the Terraform configurations, follow this guide to create the keys manually:

1.  Create a Google Cloud KMS key ring

    ```sh
    gcloud kms keyrings create "en-verification" \
      --location "us"
    ```

    Note that the "us" location is configurable. If you choose a different
    location, substitute it in all future commands.

1.  Create two signing keys - one for tokens and one for certificates:

    ```sh
    gcloud kms keys create "token-signing" \
      --location "us" \
      --keyring "en-verification" \
      --purpose "asymmetric-signing" \
      --default-algorithm "ec-sign-p256-sha256" \
      --protection-level "hsm"
    ```

    ```sh
    gcloud kms keys create "certificate-signing" \
      --location "us" \
      --keyring "en-verification" \
      --purpose "asymmetric-signing" \
      --default-algorithm "ec-sign-p256-sha256" \
      --protection-level "hsm"
    ```

    Note the "us" location is configurable, but the key purpose and algorithm
    must be the same as above.

1.  Create an encryption key for encrypting values in the database:

    ```sh
    gcloud kms keys create "database-encrypter" \
      --location "us" \
      --keyring "en-verification" \
      --purpose "encryption" \
      --rotation-period "30d" \
      --protection-level "hsm"
    ```

1.  Get the resource names to the keys:

    ```sh
    gcloud kms keys describe "token-signing" \
      --location "us" \
      --keyring "en-verification"
    ```

    ```sh
    gcloud kms keys describe "certificate-signing" \
      --location "us" \
      --keyring "en-verification"
    ```

    ```sh
    gcloud kms keys describe "database-encrypter" \
      --location "us" \
      --keyring "en-verification"
    ```

1.  Provide these values as the `TOKEN_SIGNING_KEY`, `CERTIFICATE_SIGNING_KEY`,
    and `DB_ENCRYPTION_KEY` respectively in the environment where the services
    will run. You also need to grant the service permission to use the keys.


## Observability (tracing and metrics)

The observability component is responsible for metrics. The following
configurations are available:

| Name                    | `OBSERVABILITY_EXPORTER` value  | Description
| ----------------------- | ------------------------------- | -----------
| OpenCensus Agent        | `OCAGENT`                       | Use OpenCensus.
| Stackdriver\*           | `STACKDRIVER`                   | Use Stackdriver.


## User administration

There are two types of "users" for the system:

-   **System administrator** - global system administrators are the IT
    administrators of the system. They can create new realms and edit global
    system configuration. System admins, however, do not have permissions to
    administer codes or perform realm-specific tasks beyond their creation.
    Typically a system administrator creates a realm, adds the initial realm
    admin, then removes themselves from the realm.

-   **Realm users** - users have membership in 0 or more realms, and their
    permissions in the realm determine their level of access. Most users will
    only have permission to issue codes. However, some users will have control
    over administering the realm, viewing statistics, inviting other realm
    users, or updating realm settings. See the [realm administration
    guide](realm-admin-guide.md) for more information.

When bootstrapping a new system, a default system administrator with the email
address "super@example.com" is created in the database. This user is **NOT**
created in Firebase. To bootstrap the system, log in to the Firebase console and
manually create a user with this email address and a password, then login to the
system. From there, you can create a real user with your email address and
delete the initial system user.


## Rotating secrets

This section describes how to rotate secrets in the system.

### Cookie keys

**Recommended frequency:** 30 days, on breach

The cookie keys are an array. The items at odd indicies are HMAC keys and the
items at even indicies are encryption keys. The HMAC key should be 64 bytes and
the encryption key should be 32. Even though the array is flat, each even/odd
pairing is actually a tuple:

```text
[<hmac_key_1>, <encryption_key_1>, <hmac_key_2>, <encryption_key_2>]
```

Each key is supplied to this system as base64, for example:

```sh
# "<base64_hmac_key_1>,<base64_encryption_key_1>"
export COOKIE_KEYS="ARLaFwAqBGIkm5pLjAveJuahtCnX2NLoAUz2kCZKrScUaUkEaxHSvJLVYb5yAPCc441Cho5n5yp8jdEmy6hyig==,RLjcRZeqc07s6dh3OK4CM1POjHDZHC+usNU1w/XNTjM="
```

To rotate the cookie keys, generate two new keys of the correct lengths as
specificed above and insert them **into the front** of the array. **Do not
remove the existing values from the array**, as doing so will invalidate all
existing sessions.

```text
[<NEW_HMAC_KEY>, <NEW_ENCRYPTION_KEY>, <hmac_key_1>, <encryption_key_1>, <hmac_key_2>, <encryption_key_2>]
```

Just as before, the new values should be base64-encoded:

```sh
# "<base64_hmac_key_1>,<base64_encryption_key_1>,<base64_hmac_key_2>,<base64_encryption_key_2>"
export COOKIE_KEYS="c8+OD0vpvT/FrtGAtHc1nYhtkYMhjEEHCLgzuIiKJbskAbMI7bJxSnlBMKmc2AQmo8QVAViJuKoopuSuXE7tYw==,KRN9OK/lcs/uBWKQ2/1I0g9KR/iL3/MHuCn6esI02fs=,ARLaFwAqBGIkm5pLjAveJuahtCnX2NLoAUz2kCZKrScUaUkEaxHSvJLVYb5yAPCc441Cho5n5yp8jdEmy6hyig==,RLjcRZeqc07s6dh3OK4CM1POjHDZHC+usNU1w/XNTjM="
```

Upon deploying, all _new_ sessions will use these new keys. Old sessions will be
automatically upgraded on the next visit. After a period of time that you deem
acceptable (e.g. 30d), you can remove the older keys from the end of the array.

You can use `openssl` or similar tooling to generate the secret. Note that this
is **not** preferred since it requires a human to see the plaintext secret.

```sh
openssl rand -base64 64 | tr -d "\n" # or 32
```

If you are using a secret manager, put this value in the secret manager and
provide its _reference_ in the environment. If you are not using a secret
manager, provide this value directly in the environment.


### Cross-site request forgery (CSRF) keys

**Recommended frequency:** 90 days, on breach

To rotate the key, generate a new 32-byte key. You can use `openssl` or similar:

```sh
openssl rand -base64 32 | tr -d "\n"
```

Update the `CSRF_AUTH_KEY` environment variable and re-deploy. The system [only
supports a single key for CSRF](https://github.com/gorilla/csrf/issues/65). When
you deploy the new key, any existing open HTML forms will fail to submit as an
invalid request.


### Database encryption keys

**Recommend frequency:** 30 days, on breach

These keys control application-layer encryption of secrets before they are
stored in the database. For example, this key encrypts Twilio credentials so
they are not in plaintext in the database. Note that we only use the encryption
key where encryption is appropriate. For API keys and tokens, we HMAC the values
as their plaintext values are not required.

To rotate the encryption keys, rotate them in the underlying key manager. Note
that old entries will still be encrypted with the old key. You do not need to
upgrade them so long as the older key version is still available in your key
manager.

While unlikely, this may require you to update the `DB_ENCRYPTION_KEY`
environment variable.


### API Key signature HMAC keys

**Recommended frequency:** 90 days

This key is used as the HMAC secret when signing API keys. API keys are signed
and verified using this value. Like cookies, it accepts an array of values. The
first item in the array is used to sign all new API keys, but all remaining
values are still accepted as valid. These keys should be at least 64 bytes, but 128 is recommended.

To generate a new key:

```sh
openssl rand -base64 128 | tr -d "\n"
```

Insert this new value **into the front** of the `DB_APIKEY_SIGNATURE_KEY`
environment variable:

```sh
DB_APIKEY_SIGNATURE_KEY="gSEGlr482MSTm0eGRm2VvS86iQin3+/+80ALBkKKBYgu2EJyhGkvi8BULeFQDW/qZp2y3IbKY0MUTqioG7InBZdCkisYjr8UNuA+PONxMSaw/x8m+CXF28qb2WF0OGYQIPgbOdQ7cChG3Ox5AQgWFmNwyr486lTxSM8TE7dfCfU=,oXrnYzt6MXKBB/+zZWTvkUABW8SSCAFv5Mc475sSVPGBqCf1hWvv/VmByFj/5457Ho0AVbDUiCpKnjW2Q8ZlxqRo5dJyRifwvmW2JYcpxe+Ff/d+tb2x+TwlzqEzrKVatEWX4cmMG7ZP6B1oTCw/uZPTDhgB3lerXVIBTxdAaQc="
```

Note: Removing any of the keys from this list will invalidate API keys signed by
that version.

If you are using Terraform, increment the `db_apikey_sig_hmac_count` by 1.


### API Key database HMAC keys

**Recommended frequency:** 90 days

This key is used as the HMAC secret when saving hashed API keys in the database.
Like cookies, it accepts an array of values. The first item in the array is used
to HMAC all new API keys, but all remaining values are still accepted as valid.
These keys should be at least 64 bytes, but 128 is recommended.

To generate a new key:

```sh
openssl rand -base64 128 | tr -d "\n"
```

Insert this new value **into the front** of the `DB_APIKEY_DATABASE_KEY`
environment variable:

```sh
DB_APIKEY_SIGNATURE_KEY="1do5HM96Bk9WD15BQC3qbW9e3T2V6T0DHn2i1xGJRKX8tZubxuaezivMhO3uJFEye8SITH3UFB+mo9oE0VGPiP/4TOXejfsd1g2L518itJbrK4/qNh6QMk0I04mqNtR55uvyt7G/ObADn2hQDYMVOGg/C6nLiqO+nqQ/UoUcGkA=,tJiUPEi0xS5QbykypVlquWsxQ0DAgxY41w+PkNqpoqzWQyDnEUAWFwIFUUFllqT+m0f2Kqh8EB1zjYgFcGP16O52rHer5sr4x6nsnQ/AiOHDrztJnEqGvutetHhZHLGKY0HxlxkZxcFLCmbgs6pa0vNUodrzOsCYysD7MLCSL5M="
```

Note: Removing any of the keys from this list will invalidate API keys HMACed by
that version.

If you are using Terraform, increment the `db_apikey_db_hmac_count` by 1.


### Verification Code database HMAC keys

**Recommended frequency:** 30 days

This key is used as the HMAC secret when saving verification codes in the
database. Like cookies, it accepts an array of values. The first item in the
array is used to HMAC all new verification codes, but all remaining values are
still accepted as valid. These keys should be at least 64 bytes, but 128 is
recommended.

To generate a new key:

```sh
openssl rand -base64 128 | tr -d "\n"
```

Insert this new value **into the front** of the `DB_VERIFICATION_CODE_DATABASE_KEY`
environment variable:

```sh
DB_APIKEY_SIGNATURE_KEY="g7GdsjuN+eydQIUCena2gleSHsmu46Gs+62ENViXsaV123AoVEwZ94ywpCQ2hxJ6CSBc4wXOwrxhg+psiwfp9eyBcpOFC7i98mOTLu1gxznZe943PVKl9vKJx9SgFrSnI1prWoQj85xGJKMBlM/pj608LWpZ3aIxyk0t7Sk/iWE=,G1VCqQVqe+4GD60YsqOHVgYEXN6WMh8tuF9bAfRJyt9sVk9kBWbPdhFQVUdCqoE3cckSsxz7LMApiN1/2jbwG3qkTicx4YuwQMUDVg2Stdv0L2kvek/+MYcA0lVYaNZWBJCSgmaMzjzOGW/BsbR/ssX1WhGI9aVoGpFQMiEJaVE="
```

Note: Removing any of the keys from this list will invalidate verification codes
HMACed by that version. However, given verification a verification code's
lifetime is short, it is probably safe to remove the key beyond 30 days.

If you are using Terraform, increment the `db_verification_code_hmac_count` by 1.


### Token signing keys

**Recommended frequency:** automatic

The system automatically rotates token signing keys every 30 days.

When bootstrapping a new system from scratch, it can take up to 5 minutes for
the initial token signing key to become available. To expedite this process, you
can manually invoke the `rotation` scheduler job:

```sh
gcloud scheduler jobs run "rotation" \
  --project "${PROJECT_ID}"
```


### Certificate signing keys

**Recommended frequency:** on demand

If you are using system keys, the system administrator will handle rotation. If
you are using realm keys, you can generate new keys in the UI.


### Cacher HMAC keys

**Recommended frequency:** 90 days, on breach

This key is used as the HMAC key to named values in the cacher. For example, API
keys are cached in the cacher for a few minutes to reduce load on the database.
We do not want the cacher to have plaintext API keys, so the values are HMACed
before being written (and HMACed on lookup). This prevents a server operator
with access to the cacher (e.g. Redis) from seeing plaintext data about the
system. The data is hashed instead of encrypted because we only need a
deterministic value to lookup.

To generate a new key:

```sh
openssl rand -base64 128 | tr -d "\n"
```

Use this value as of the `CACHE_HMAC_KEY` environment variable:

```sh
CACHE_HMAC_KEY="RBwXRppIqscSWxSsP/e52AsPsab4jW7lL5DJSw3uZfTbwgGXj3IV/iWx0ZGjyvY0GB3kupK7qbaDZBGsxxqABT4thujJkx6kAiAabH4kz5qPwoPNGK2M9KW9TX5jM3dnX7smPzlL+Hg8ijxczceDCeQF44cys+3rdWaDdC6kHec="
```

Note: Changing this value will invalidate any existing caches. Most caches are
small and are automatically re-built on demand, so occasional rotation is likely
fine for this system.


### Rate limit HMAC keys

**Recommended frequency:** 90 days, on breach

This key is used as the HMAC key to named values in the rate limit. For example,
API keys and IP addresses are rate limited. We do not want the rate limiter to
have those values in plaintext, so the values are HMACed before being written
(and HMACed on lookup). This prevents a server operator with access to the rate
limiter (e.g. Redis) from seeing plaintext data about the system. The data is
hashed instead of encrypted because we only need a deterministic value to
lookup.

To generate a new key:

```sh
openssl rand -base64 128 | tr -d "\n"
```

Use this value as of the `RATE_LIMIT_HMAC_KEY` environment variable:

```sh
RATE_LIMIT_HMAC_KEY="43+ViAkv7uHYKjsXhU468NGBZrtlJWtZqTORIiY8V6OMsLAZ+XmUF5He/wIhRlislnteTmChNi+BHveSgkxky81tpZSw45HKdK+XW3X5P7H6092I0u7H31C0NaInrxNxIRAbSw0NxSIKNbfKwucDu1Y36XjJC0pi0wlJHxkdGes="
```


## SMS with Twilio

The verification server can optionally be configured to send SMS messages with
app deep-links for the verification codes. This removes the need for a case
worker to dictate a code over the phone, but requires the use of [Twilio](https://twilio.com) to
send SMS text messages. To get started:

1.  [Create an account on Twilio](https://www.twilio.com/try-twilio).

1.  [Purchase](https://support.twilio.com/hc/en-us/articles/223135247-How-to-Search-for-and-Buy-a-Twilio-Phone-Number-from-Console)
    or
    [transfer](https://support.twilio.com/hc/en-us/articles/223179348-Porting-a-Phone-Number-to-Twilio)
    a phone number from which SMS text messages will be sent.

    Note: To reduce the chance of your SMS messages being flagged as spam, we
    strongly recommend registering a toll-free SMS number or SMS short code.

1.  Find your Twilio **Account SID** and **Auth token** on your [Twilio dashboard](https://twilio.com/dashboard).

1.  Go to the realm settings page on the **SMS** tab, enter these values, and
    click save.

1.  Case workers will now see an option on the **Issue code** page to enter a
    phone number. This is _always_ optional in case the patient does not have an
    SMS-enabled cell phone.

[gcp-kms]: https://cloud.google.com/kms

## Identity Platform setup

The verification server uses the Google Identity Platform for authorization.

1. Visit the [Google Identity Platform MFA](https://console.cloud.google.com/customer-identity/mfa) page. Ensure the identity platform is enabled for your project and ensure 'Multi-factor-authorization' is toggled on. Here you may also register test phone numbers for development.

2. Navigate to https://firebase.corp.google.com/u/0/project/{your project id}/authentication/emails to modify the email templates sent during password reset / verify email. Customize the link to your custom domain (if applicable) and direct it to '/login/manage-account' to use the custom password selection. You may also customize the text of the email if you wish.

3. Visit [Google Identity Platform Settings](https://console.cloud.google.com/customer-identity/settings) and ensure that 'Enable create (sign-up)' and 'Enable delete' are unchecked. This system is intended to be invite-only and these flows are handled by administrators.

## End-to-end test runner

Log in as a system admin and view realms, select the `e2e-test-realm`. If this
realm has not yet been created, wait a few minutes. The e2e runner executes
every 15 minutes.

![system admin realms](images/e2e/image01.png)

Join the realm

![join e2e-test-realm](images/e2e/image02.png)

Select the "Back to e2e-test-realm" link

![back to e2e-test-realm](images/e2e/image03.png)

Select `Signing Keys` from the drop down menu

![select signing keys](images/e2e/image04.png)

Create a new (initial) realm specific signing key

![create realm signing keys](images/e2e/image05.png)

Set the issuer and audience values (`iss`/`aud`). The suggested issuer is
the reverse DNS of your verification server plus the realm (`e2e-test-realm`)
and the suggested audience value is the reverse DNS of your key server's exposure
service. You will need this information later when configuring the key server.

![create realm signing keys](images/e2e/image06.png)

Now, upgrade to realm specific signing keys for this realm.

![create realm signing keys](images/e2e/image07.png)

Moving on to the key server. Connect to your key server instance
and run the admin console. Create a new verification key.

![create key server verification key](images/e2e/image08.png)

Set the name, issuer, audience and JWKS URI in the configuration.

![verification key configuration](images/e2e/image09.png)

Navigate back to the admin console home, and then click back into the
newly created verification key config. Refresh this page until the JWKS
importer has picked up the public key.

When that is ready, navigate back to the admin console home and create
a new authorized health authority.

![new health authority](images/e2e/image10.png)

Set the Health Authority ID to `e2e-test-only`, set the regions to
something your system is not using, and select the matching certificate
value.

![health authority config](images/e2e/image11.png)

Wait 5 minutes (cache refresh at exposure service), and then force the end to end
workflows to run (verification Cloud Scheduler).

![cloud scheduler](images/e2e/image12.png)

If the workflows move to `success`, then you have done everything correctly!

## Architecture

![diagram of layout](images/architecture/go-diagrams/diagram.png)
