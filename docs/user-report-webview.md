<!-- TOC depthFrom:1 -->

- [Access](#access)
- [Initiate](#initiate)
- [Client side throttling](#client-side-throttling)
- [Validate](#validate)

<!-- /TOC -->

# Access

To access the user report webview you need

* A `DEVICE` level API key
* The base URL for the EN Express redirect service

# Initiate

Open a webview with a POST (recommended) or GET request to the base URL for the 
EN Express redirect service for your installation, at the path `/report`.

Pass in these required headers:

* `X-API-Key`: <your api key>
* `X-Nonce`: 256 bytes of random data, base64 encoded (base64 URL encoding recommended)

This will establish a session with the server and render a form for the user to fill out.

The verification code / link will be sent to the user's mobile phone number.

# Client side throttling

If the client has determined that this particular device has requested user-report codes
too frequently, the client has the option to pass the `X-Nonce` header with no value.
In this case, the server will render an appropriate error.

# Validate

The `nonce` that is generated when loading the webview should be passed 
to the next call to `/api/verify` or forgotten after 24 hours.
