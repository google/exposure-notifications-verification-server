# tek-verification-server

YO!

This shows a sample Web interface for issuing a "PIN CODE"

That PIN code has some information about TEKs that might one day be uploaded
with that pin.

When a "mobile app" presents that PIN plus the HMAC of the TEKs, this server
will verify the PIN and sign the claims in a JWT, with optional
additional metadata at the direction of the PHA.

If you wanted to run this yourself, you just need to create an asymmetric
ESCDA P256  signing key and swap out the resource ID in cmd/server/main.go

Then

1. go run ./cmd/server
2. visit http://localhost:8080
3. Configure and issue a pin (keep the server running)
4. go run `./cmd/client -pin ISSUEDPIN`
5. Visit https://jwt.io
6. Copy the validation payload into the left side
7. Get the public key from KMS and copy to the right if you want to verify sig.
