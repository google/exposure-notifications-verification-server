# Using the Cloud SQL Proxy

By default, the Cloud SQL Postgres database only accessible via a private IP
address from authorized networks. To access the database locally to perform
administrative operations or bootstrap the system, you must use the
[`cloud_sql_proxy`][proxy].

The proxy, as its name implies, creates a tunnel between your machine and the
private network upon which the Cloud SQL instance is running. By default, this
tunnel maps to `127.0.0.1:PORT`. That means if you make a request to
`127.0.0.1:PORT` while the proxy is running, the proxy will forward that request
to the Cloud SQL instance on its private IP address automatically:

```text
127.0.0.1:5432 -> 10.0.3.1:5432 (private)
```

## Setup

1.  [Download and install the proxy][proxy-install] for your operating system.

1.  Choose and configure an [authentication option to the proxy][proxy-auth].
    The most preferred options are:

    - Credentials from the gcloud Cloud SDK client (local machine)
    - Credentials from a Compute Engine instance (cloud machine)


## Usage

1.  Start the Cloud SQL proxy with the name of your instance:

    ```shell
    cloud_sql_proxy \
      -dir "${HOME}/sql" \
      -instances "<instance>=tcp:5432"
    ```

    Where `<instance>` is full ID of the instance. It should resemble:

    ```text
    my-project:us-central1:en-verification
    ```

1.  If successful, the proxy will "take over" the terminal session. Open a new
    window or tab to continue. **Do not close the tab as it is running the
    proxy!**

1.  In a new tab or window, configure your database parameters as the
    environment variables required by the system. All parameters are the same
    **except**:

    -   `DB_HOST=127.0.0.1` - this value should be `127.0.0.1` because you are
        going through the proxy.

    -   `DB_SSLMODE=disable` - the proxy is running on localhost, so there's no
        TLS; the connection between the proxy and Cloud SQL is still secured via
        TLS.

1.  Execute any database commands as normally.


[proxy]: https://cloud.google.com/sql/docs/postgres/sql-proxy
[proxy-install]: https://cloud.google.com/sql/docs/postgres/sql-proxy#install
[proxy-auth]: https://cloud.google.com/sql/docs/postgres/sql-proxy#authentication-options
