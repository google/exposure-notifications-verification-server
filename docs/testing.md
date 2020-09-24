# Manual testing and invocation

This document describes the process for manually testing the system.

1.  Create an Admin API and a Device API key via the web UI.

1.  Request a verification code using the **Admin** API key:

    ```sh
    go run ./tools/get-code \
      -type "confirmed" \
      -onset "2020-08-01" \
      -apikey "ADMIN_API_KEY"
    ```

    The code will be 6-10 digits depending on your configuration. Save this code
    as you will need it in a moment.

1.  Exchange the verification _code_ for the verification _token_ using the
    **Device** API key:

    ```sh
    go run ./tools/get-token \
      -apikey "DEVICE_API_KEY" \
      -code "CODE_FROM_STEP_2"
    ```

    This will return a token. The token will be a JWT token, for example:

    ```text
    eyJhbGciOiJFUzI1NiIsImtpZCI6InYxIiwidHlwIjoiSldUIn0.eyJhdWQiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJleHAiOjE1OTUwMTk0NjEsImp0aSI6Im5BbVdJKzVnZDRuSG0wcnJiOGRGWUVwUExDdFpaK2dMOXZ5YjVCcDJIdmVHTndmeHV5ZS9rU2x2Q2NhSGovWEwrelh5K1U1L3JpdFh1SGt1eGtvc3dLam13ZlJ0ZUpRQWpqeEdYazV5cFpPeENySGM2Z1ZVZTdxdVVNZFVkRkpBIiwiaWF0IjoxNTk0OTMzMDYxLCJpc3MiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJzdWIiOiJsaWtlbHkuMjAyMC0wNy0xMCJ9.mxMsCwRUc6AtHNNjf_xjlxT4xJrwK2b1OkOvyWDmSKxJunaOBO_j9s4SCG_b3TbZn2eAPeqG8zNSu_YUzS5GYw
    ```

    Save this token as you will need it for the next step.

1.  Exchange the verification _token_ with a verification _certificate_ with the
    HMAC of the TEKs you plan to upload:

    ```sh
    go run ./tools/get-certificate \
      -apikey "DEVICE_API_KEY" \
      -token "VERIFICATION_TOKEN" \
      -hmac "HMAC_OF_TEKS"
    ```

    The certificate will also be a JWT, for example:

    ```text
    eyJhbGciOiJFUzI1NiIsImtpZCI6InYxIiwidHlwIjoiSldUIn0.eyJyZXBvcnRUeXBlIjoibGlrZWx5Iiwic3ltcHRvbU9uc2V0SW50ZXJ2YWwiOjI2NTcyMzIsInRyaXNrIjpbXSwidGVrbWFjIjoiMnUxbkh0NVdXdXJKeXRGTEYzeGl0TnpNOTlvTnJhZDJ5NFlHT0w1M0FlWT0iLCJhdWQiOiJleHBvc3VyZS1ub3RpZmljYXRpb25zLXNlcnZlciIsImV4cCI6MTU5NDkzNDA2NCwiaWF0IjoxNTk0OTMzMTY0LCJpc3MiOiJkaWFnbm9zaXMtdmVyaWZpY2F0aW9uLWV4YW1wbGUiLCJuYmYiOjE1OTQ5MzMxNjN9.gmIzjVUNLtmGHCEybx7NXw8NjTCKDBszUHeE3hnY9u15HISjtjpH2zE_5ZXk2nlRQT9OFQnIkogO8Bz4zLbf_A
    ```
