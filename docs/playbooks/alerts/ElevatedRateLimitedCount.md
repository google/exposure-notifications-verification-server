# Elevated Rate Limited Count Alert

This alert fires when the condition of at least one request per second has been rate limited for a sustained period of more than 5 minutes. If you are paged for this alert the first goal is to make sure the system isn’t under attack.

Start by using the following query to check where the requests are being rate limited on Metrics Explorer:

```
fetch generic_task
| metric
    'custom.googleapis.com/opencensus/en-verification-server/ratelimit/limitware/request_count'
| align rate(1m)
| every 1m
| group_by [resource.job],
    [value_request_count_aggregate: aggregate(value.request_count)]
```

Next you will want to confirm how the requests are being rate limited. There are 3 possible levels of rate limiting:
* API Keys
* User ID
* IP Address.

A simple way to confirm this information is accessing Logs Explorer on Cloud Console and filtering by Debug messages.
You will be able to find payload messages like `"limiting by realm from apikey"`, `"limiting by user"` or `“limiting by ip”`.

```
severity=DEBUG
jsonPayload.message="limiting by realm from apikey"
```

See what to do in each scenario:

## API keys
This will be the first level where requests will be rate limited, if needed.  If an API key is provided,
the system rate limits by the realm associated with the API key, by client IP address.
API keys are specified as the `X-API-Key` HTTP header, but not all endpoints require that header.

When requests are rate limited by API keys the logs will contain a payload
message saying `“limiting by realm from apikey”`. If you noticed these entries
on the logs, please contact a realm administrator of the Realm ID listed in the
logs.

## User ID
This is the second level of rate limiting and affects users who are directly authenticated to the system through the web interface.
Since a user can be a member of multiple realms, the enforcement mechanism here is per-user, not per-realm.

When requests are rate limited by user ID the logs will contain a payload message saying `“limiting by user”` and the logs will contain the user ID.
You could verify which user is when accessing https://encv.org/realm/users/USERID.

If you believe this user is abusing the system you can remove the user from the realm or delete the user, if needed.

## IP address
If no authentication information is present, rate limiting is applied by the requestor's IP address.
When requests are rate limited by IP address the logs will contain a payload message saying `“limited by ip”` and the IP address will also be logged.
It is possible to block the IP address from accessing the system, but first make sure it doesn't match the following scenario.

### Scenario: app users report being rate limited
Applications authenticate via an API key and the rate limit for API keys is enforced by realm, by client IP. The most likely scenario is that all these users are being NATed through the same outbound IP address (e.g. a university computer network). There is no mitigation the verification server can provide in this case.
