# EN Express | SMS Verification / Deep Link Handling

This document applies to external developers that are creating custom
exposure notifications applications that are deployed along side Exposure Notifications
Express (ENX), and where that public health jurisdiction is using SMS to send
verification codes.

## Background

The verification system is configured with a "redirect domain" that is
used as part of the verification link protocol. This is separate from
the [ENS protocol specification](ens-spec.md).

## Examples in this doc

For purposes of this document, we use the ENX redirect domain that is
in use in the United States: `en.express`.

Each state is assigned a custom subdomain for this purpose, based on their
2 level ISO region. Here, we'll use Washington state, `us-wa`.

## Change your app

When using a custom application and iOS EN Express in the same public
health jurisdiction, some changes need to be made to the custom app (both
the iOS and Android version).

For the `ens://` protocol, this protocol was not used on Android and
on iOS a custom application cannot claim that protocol (the OS has
claimed it).

Because of this, the custom application needs to handle the appropriate
`https` link that contains the verification code.

Your custom application needs to register the jurisdiction specific
verification link domain, for example registering `https://us-wa.en.express`
as a universal link.

In the verification server, configure your custom application handlers.
Once this is done the server will host the appropriate .well-known links.
This work for both iOS and Android apps, for example (different fake app):

Sample iOS universal link metadata

```shell
❯ curl -A "iphone" "https://us-moo.en.express/.well-known/apple-app-site-association"
{
	"applinks": {
		"details": [
			{
				"appID": "ABCD1234.com.google.test.application",
				"paths": ["*"]
			}
		]
	}
}
```

Sample Android universal link metadata

```shell
❯ curl "https://us-moo.en.express/.well-known/assetlinks.json"
[
	{
		"relation": [
			"delegate_permission/common.handle_all_urls"
		],
		"target": {
			"namespace": "android_app",
			"package_name": "gov.moosylvania.enx",
			"sha256_cert_fingerprints": [
				"A0:78:81:44:73:27:91:F3:0F:38:EE:98:D6:95:BD:B5:4D:3D:9C:81:A2:90:0B:15:59:DC:C3:DB:B5:B6:93:93",
				"45:0B:F6:29:B0:65:82:D1:C8:3A:8B:6F:91:54:02:C0:92:C6:B6:23:C5:D2:49:20:A5:F1:5A:3D:8C:1B:6E:65",
				"AA:14:E8:9D:37:4C:B3:6C:80:E7:9F:73:BC:9A:01:D3:16:77:DC:C6:91:AA:DE:A1:5F:73:74:11:B3:36:A3:91"
			]
		}
	}
]
```

For the iOS universal links, if the custom EN app is installed that handles the `us-wa.en.express` domain, then the Washington app will get a chance to handle it. It is up to that
developer to handle the link correctly in the application.

If the Washington app is not installed, then when the user clicks the link on iOS, the server will redirect them to `ens://` which will be picked up by iOS ENX

```shell
❯ curl -A "iphone" "https://us-um.en.express/v?c=SECRETCODE"
<a href="ens://v?c=SECRETCODE&amp;r=US-UM">See Other</a>.
```

The same is true of a custom Android application needing to handle these links.

In this instance, it is important to note that for EN Express, the verification
code embedded in the link is a 16 character alphanumeric code. An example link
would be

```
https://us-wa.en.express/v?c=1234abcd5678efgh
```
