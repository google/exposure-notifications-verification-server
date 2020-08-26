# Local development

This guide covers how to develop the system locally. Note that the configuration
options described in this page are optimized for local development and may not
represent best practices.

1.  Install [gcloud](https://cloud.google.com/sdk).

1.  Create a Google Cloud project using the Cloud Console. Set your **Project
    ID** as an environment variable:

    ```sh
    export PROJECT_ID="..."
    ```

    Note this is the project _ID_, not _name_ or _number_.

1.  Configure gcloud:

    ```sh
    gcloud auth login && \
    gcloud auth application-default login && \
    gcloud auth application-default set-quota-project "${PROJECT_ID}"
    ```

1.  Install the [Firebase CLI](https://firebase.google.com/docs/cli).

1.  Configure the Firebase CLI:

    ```text
    firebase login
    ```

    Use the same Google credentials as you used in the previous steps.

1.  Create a Google Cloud KMS key ring and two signing keys:

    ```sh
    gcloud kms keyrings create "signing" \
      --location "us"

    gcloud kms keys create "token-signing" \
      --location "us" \
      --keyring "signing" \
      --purpose "asymmetric-signing" \
      --default-algorithm "ec-sign-p256-sha256"

    gcloud kms keys create "certificate-signing" \
      --location "us" \
      --keyring "signing" \
      --purpose "asymmetric-signing"  \
      --default-algorithm "ec-sign-p256-sha256" \
    ```

    To get the resource names to the keys (for use below):

    ```sh
    gcloud kms keys describe "token-signing" \
      --location "us" \
      --keyring "signing"

    gcloud kms keys describe "certificate-signing" \
      --location "us" \
      --keyring "signing"
    ```

1.  Create a `.env` file with your configuration. This will aid future
    development since you can `source` this file instead of trying to find all
    these values again.

    ```sh
    # Create a file named .env with these contents
    export PROJECT_ID="YOUR_PROJECT_ID" # TODO: replace
    export GOOGLE_CLOUD_PROJECT="${PROJECT_ID}"

    # Get these values from the firebase console
    export FIREBASE_API_KEY="TODO"
    export FIREBASE_PROJECT_ID="${PROJECT_ID}"
    export FIREBASE_MESSAGE_SENDER_ID="TODO"
    export FIREBASE_APP_ID="TODO"
    export FIREBASE_MEASUREMENT_ID="TODO"
    export FIREBASE_AUTH_DOMAIN="${PROJECT_ID}.firebaseapp.com"
    export FIREBASE_DATABASE_URL="https://${PROJECT_ID}.firebaseio.com"
    export FIREBASE_STORAGE_BUCKET="${PROJECT_ID}.appspot.com"
    export FIREBASE_PRIVACY_POLICY_URL="TODO"
    export FIREBASE_TERMS_OF_SERVICE_URL="TODO"

    # Populate these with the resource IDs from above. These values will be of
    # the format:
    #
    # projects/ID/locations/us/keyRings/signing/cryptoKeys/token-signing/cryptoKeyVersions/1Z
    export TOKEN_SIGNING_KEY="TODO"
    export CERTIFICATE_SIGNING_KEY="TODO"

    # Disable local observability
    export OBSERVABILITY_EXPORTER="NOOP"

    # Configure a CSRF auth key. Create your own with `openssl rand -base64 32`.
    export CSRF_AUTH_KEY="RcCNhTkS9tSDMSGcl4UCa1FUg9GmctkJpdI+eqZ+3v4="

    # Configure cookie encryption, the first is 64 bytes, the second is 32.
    # Create your own with `openssl rand -base64 NUM` where NUM is 32 or 64
    export COOKIE_KEYS="ARLaFwAqBGIkm5pLjAveJuahtCnX2NLoAUz2kCZKrScUaUkEaxHSvJLVYb5yAPCc441Cho5n5yp8jdEmy6hyig==,RLjcRZeqc07s6dh3OK4CM1POjHDZHC+usNU1w/XNTjM="

    # Use an in-memory key manager for encrypting values in the database. Create
    # your own encryption key with `openssl rand -base64 64`.
    export KEY_MANAGER="IN_MEMORY"
    export DB_ENCRYPTION_KEY="O04ZjG4WuoceRd0k2pTqDN0r8omr6sbFL0U3T5b12Lo="

    # Database HMAC keys - these should be at least 64 bytes, preferably 128
    # Create your own with `openssl rand -base64 128`.
    export DB_APIKEY_DATABASE_KEY="RlV/RBEt0lDeK54r8U9Zi7EDFZid3fiKM2HFgjR9sZGMb+duuQomjGdNKYnzrNyKgeTBcc1V4qVs6fBrN6IFTLbgkp/u52MGhSooAQI4EuZ6JFuyxQBeu54Ia3mihF111BMcCWpHDg2MAh8k8f669plEQaqoQFg3GThP/Lx1OY0="
    export DB_APIKEY_SIGNATURE_KEY="HFeglmupbtv/I2X04OQRl1V7mcvfAXuv8XtmIFYV6aYsPuwQVFtXDlfFrjouYT2Z6kYln7B90RcutHJNjpPDRkyBQ28HtWmid3dr0tpJ1KiiK5NGG7JS9mU8fCvEYklw5RV+1f8qN13nWzHpW8/RQw9rR/vQGy90yL5/aydBuVA="
    export DB_VERIFICATION_CODE_DATABASE_KEY="YEN4+tnuf1DzQPryRzrPVilqT0Q2TO8IIg3C8prvXWGAaoABOWACl79hS40OneuaU8GsQHwhJ13wM2A5ooyOq+uqxCjrqVJZZXPU5xzl/6USEYAp4z2b0ZYrfkx2SRk1o9HfFi1RMqpaBf1TRIbsNOK9hNRG3nS2It49y6mR1ho="

    # Enable dev mode
    export DEV_MODE=1
    export DB_DEBUG=1
    ```

1.  Source the `.env` file. Do this each time before you start the server:

    ```sh
    source .env
    ```

1.  Start the database:

    ```sh
    eval $(./scripts/dev init)
    ./scripts/dev dbstart
    ```

1.  Run any migrations:

    ```sh
    ./scripts/dev dbmigrate
    ```

1.  (Optional) Seed the database with fake data:

    ```sh
    go run ./tools/seed
    ```

    This will create some default users like `admin@example.com` and
    `user@example.com` for local development. Check the seed file for more
    details on what is created.

1.  Start the server:

    ```sh
    go run ./cmd/server
    ```

    Open your browser to http://localhost:8080.


## Tips

### Bypass MFA

Register a
[test-phone-number](https://cloud.google.com/identity-platform/docs/test-phone-numbers)
for your account by visiting:

    https://console.cloud.google.com/customer-identity/mfa?project=${PROJECT_ID}

This will skip the actual sending of SMS codes for 2-factor auth and allow you
to instead set a static challenge response code. Do not do this in production.
