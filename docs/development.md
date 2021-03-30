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

1.  Change directory into this repository:

    ```text
    cd /path/to/exposure-notifications-verification-server
    ```

1.  Bootstrap the local key management system:

    ```text
    go run ./tools/gen-keys
    ```

    This will output some environment variables. **Save these environment
    variables for the next step!**

    The default development setup uses a local, on-disk key manager to persist
    across server restarts. The production installation recommends a hosted key
    management service like Google Cloud KMS. It is possible to use Google Cloud
    KMS locally by following the instructions in the production setup guide.

1.  Create a `.env` file with your configuration. This will aid future
    development since you can `source` this file instead of trying to find all
    these values again.

    ```sh
    # Google project configuration.
    export PROJECT_ID="TODO"
    export GOOGLE_CLOUD_PROJECT="${PROJECT_ID}"

    # Get these values from the firebase console.
    export FIREBASE_API_KEY="TODO"
    export FIREBASE_PROJECT_ID="${PROJECT_ID}"
    export FIREBASE_MESSAGE_SENDER_ID="TODO"
    export FIREBASE_APP_ID="TODO"
    export FIREBASE_MEASUREMENT_ID="TODO"
    export FIREBASE_AUTH_DOMAIN="${PROJECT_ID}.firebaseapp.com"
    export FIREBASE_DATABASE_URL="https://${PROJECT_ID}.firebaseio.com"
    export FIREBASE_STORAGE_BUCKET="${PROJECT_ID}.appspot.com"

    # Disable local observability.
    export OBSERVABILITY_EXPORTER="NOOP"

    # Configure cookie encryption, the first is 64 bytes, the second is 32.
    # Create your own values with:
    #
    #     openssl rand -base64 NUM
    #
    # where NUM is 32 or 64, respectively.
    export COOKIE_KEYS="ARLaFwAqBGIkm5pLjAveJuahtCnX2NLoAUz2kCZKrScUaUkEaxHSvJLVYb5yAPCc441Cho5n5yp8jdEmy6hyig==,RLjcRZeqc07s6dh3OK4CM1POjHDZHC+usNU1w/XNTjM="

    # Configure cache and cache HMAC. Create your own values with:
    #
    #     openssl rand -base64 128
    #
    export CACHE_TYPE="IN_MEMORY"
    export CACHE_HMAC_KEY="/wC2dki5Z+To9iFwUamINtHIMOH/dME7e5Gy+9h3WTDBhqeeSYkqduZRjcZWwG3kPMdiWAdBxxop5wB+BHTBnSlfVVmy8qKVNv+Wf5ywgxV7SbB8bjNQBHSpn7aC5RxR6nkEsZ2w2fUhTJwD9q+MDo6TQvf+8OXEPrV1SXWNHrs="

    # Configure rate limiter. Create your own values with:
    #
    #     openssl rand -base64 128
    #
    export RATE_LIMIT_TYPE="MEMORY"
    export RATE_LIMIT_HMAC_KEY="/wC2dki5Z+To9iFwUamINtHIMOH/dME7e5Gy+9h3WTDBhqeeSYkqduZRjcZWwG3kPMdiWAdBxxop5wB+BHTBnSlfVVmy8qKVNv+Wf5ywgxV7SbB8bjNQBHSpn7aC5RxR6nkEsZ2w2fUhTJwD9q+MDo6TQvf+8OXEPrV1SXWNHrs="

    # Configure certificate key management. The CERTIFICATE_SIGNING_KEY should
    # be the value output in the previous step.
    export CERTIFICATE_KEY_MANAGER="FILESYSTEM"
    export CERTIFICATE_KEY_FILESYSTEM_ROOT="$(pwd)/local"
    export CERTIFICATE_SIGNING_KEY="TODO" # (e.g. "/system/certificate-signing/1122334455")

    # Configure sms key management.
    export SMS_KEY_MANAGER="FILESYSTEM"
    export SMS_KEY_FILESYSTEM_ROOT="$(pwd)/local"

    # Configure token key management. The TOKEN_SIGNING_KEY should be the value
    # output in the previous step.
    export TOKEN_KEY_MANAGER="FILESYSTEM"
    export TOKEN_KEY_FILESYSTEM_ROOT="$(pwd)/local"
    export TOKEN_SIGNING_KEY="TODO" # (e.g. "/system/token-signing/1122334455")

    # Configure the database key manager. The DB_KEYRING and DB_ENCRYPTION_KEY
    # should be the values output in the previous step.
    export DB_KEY_MANAGER="FILESYSTEM"
    export DB_KEY_FILESYSTEM_ROOT="$(pwd)/local"
    export DB_KEYRING="TODO" # (e.g. "/realm")
    export DB_ENCRYPTION_KEY="TODO" # (e.g. "/system/database-encryption")

    # Use an in-memory key manager for encrypting values in the database. Create
    # your own encryption key with `openssl rand -base64 64`.
    export KEY_MANAGER="IN_MEMORY"
    export DB_ENCRYPTION_KEY="O04ZjG4WuoceRd0k2pTqDN0r8omr6sbFL0U3T5b12Lo="

    # Database HMAC keys - these should be at least 64 bytes, preferably 128.
    # Create your own with:
    #
    #     openssl rand -base64 128
    #
    export DB_APIKEY_DATABASE_KEY="RlV/RBEt0lDeK54r8U9Zi7EDFZid3fiKM2HFgjR9sZGMb+duuQomjGdNKYnzrNyKgeTBcc1V4qVs6fBrN6IFTLbgkp/u52MGhSooAQI4EuZ6JFuyxQBeu54Ia3mihF111BMcCWpHDg2MAh8k8f669plEQaqoQFg3GThP/Lx1OY0="
    export DB_APIKEY_SIGNATURE_KEY="HFeglmupbtv/I2X04OQRl1V7mcvfAXuv8XtmIFYV6aYsPuwQVFtXDlfFrjouYT2Z6kYln7B90RcutHJNjpPDRkyBQ28HtWmid3dr0tpJ1KiiK5NGG7JS9mU8fCvEYklw5RV+1f8qN13nWzHpW8/RQw9rR/vQGy90yL5/aydBuVA="
    export DB_VERIFICATION_CODE_DATABASE_KEY="YEN4+tnuf1DzQPryRzrPVilqT0Q2TO8IIg3C8prvXWGAaoABOWACl79hS40OneuaU8GsQHwhJ13wM2A5ooyOq+uqxCjrqVJZZXPU5xzl/6USEYAp4z2b0ZYrfkx2SRk1o9HfFi1RMqpaBf1TRIbsNOK9hNRG3nS2It49y6mR1ho="

    # Configure database pooling.
    export DB_POOL_MIN_CONNS="2"
    export DB_POOL_MAX_CONNS="10"

    # Enable dev mode. Do not enable dev mode or database dev mode in production
    # environments.
    export LOG_MODE="development"
    export LOG_LEVEL="debug"
    export DEV_MODE="true"
    export DB_DEBUG="true"
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
    ./scripts/dev dbseed
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

### Feature Flags

For functionality that is ready for test environments but not yet ready for default
on in production, that functionality should be guarded by a feature flag.

Define the new flag in [pkg/config/feature_config.go](https://github.com/google/exposure-notifications-verification-server/blob/main/pkg/config/feature_config.go).

The feature config is available in the config struct of all servers.

For the UI server (`server`), flags values are automatically added to the
template variables for use in the HTML templates.

If a whole controller should be controlled by this flag, you should install the
[enabled](https://github.com/google/exposure-notifications-verification-server/blob/main/pkg/controller/middleware/enabled.go)
middleware with the correct boolean value at route configuration time.
