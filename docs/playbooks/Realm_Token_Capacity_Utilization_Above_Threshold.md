# Realm Token Capacity Utilization Above Threshold

This alerts fires when we are out of Token quota. We fire this alert to notify system admins because we can't notify realm admins.

To triage, check the following:

* Metric for realm capacity
* Contact realm admin that this spike is expected

```
generic_task :: custom.googleapis.com/opencensus/en-verification-server/api/issue/realm_token_capacity_latest | align rate() | every 1m | group_by [metric.realm], [max(value.realm_token_capacity_latest)]
```

Note that metrics only show the realm ID (an integer) because realm names are PII.

TODO(marilia): Add way to turn Realm ID into Realm Name.

To mitigate:

Once you know what realm it is and that this is legitimate use:


* Login to the realm (Click Join Realm in System Admin)
* Under your name click "Manage Realm - Settings"
* Then select the subheader "Abuse prevention"
* Add to the quota by giving a temporary burst of the amount you think this realm needs.


TODO(thegrinch): Add logging of new burst capacity.

