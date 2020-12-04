# Tips for Troubleshooting

## Google Cloud Console

Link to [Google Cloud Console][].

Tips to quickly navigate around Cloud Console:

1. You can "pin" commonly used pages to the top of the left sidebar.
   Hover your mouse on a page in the sidebar and there's a pin button.
2. At the top of the page you can search the name of the page you want
   to access.

## Using Logs Explorer

[Using the Logs Explorer][]

Logs produced by the verification server is in `stderr` log. You can
filter them out using

```
resource.type="cloud_run_revision"
logName="projects/encv-prod/logs/run.googleapis.com%2Fstderr"
```

You can further narrow down using filters on `jsonPayload`. E.g. the
following query will give you log lines from e2e test:

```
resource.type="cloud_run_revision"
logName="projects/encv-prod/logs/run.googleapis.com%2Fstderr"
jsonPayload.caller=~"e2e.go"
```

Check [Logging query language][] to see other tips.

## Using Metrics Explorer

The Metrics Explorer is a useful tool to see the stats of various aspect
of the server.

You can find Metrics Explorer in Google Cloud Console, either by search
"metric explorer" in the search bar at the top, or using the link in the
side bar (Under OPERATIONS -> Monitoring -> Metrics Explorer).

By default you can edit the query using the UI (See [Using Metrics
Explorer][]), useful for quick poking around.

Since all of our dashboards/alerts are defined in Monitoring Query
Language (mql), you should also be familiar with constructing the query
using the Query Editor (See [Using the Query Editor][]).

## Monitoring Query Language (mql)

This section is intended to give a quick overview of the Monitoring
Query Language used by our monitoring/dashboard/alerting. You are
encouraged to read the [MQL reference][] and [MQL examples][] to get
more details.

An MQL query looks like this:

```
generic_task ::
custom.googleapis.com/opencensus/en-verification-server/api/issue/request_count
| filter metric.result == "OK"
| [metric.realm]
| sum
```

Line by line explanation

1. `generic_task`: The "resource type" of the query. All of our
   custom metrics (metrics our code exports) uses `generic_task`
   resource type. Google Cloud Monitoring also provide built-in metrics
   for all running services, examples include:
   - `cloud_run_revision`: metrics provided by Cloud Run.
   - `redis_instance`: metrics provided by Cloud Mememorystore Redis.
   - `https_lb_rule`: metrics provided by Cloud Load Balancer
   - `cloudsql_database`: metrics provided by Cloud SQL
2. `custom.googleapis.com/...`: The name of the metric.
   - Our custom metrics are prefixed with
     `custom.googleapis.com/opencensus/`.
     - Tip: Metrics defined in our code has a prefix of
       `.../opencensus/en-verification-server/`. We
       also have other metrics provided by OpenCensus to gain visibility
       into several parts of the external libraries.
       - `.../opencensus/go.sql/`
       - `.../opencensus/grpc.io/`
       - `.../opencensus/opencensus.io/http/`
       - `.../opencensus/redis/`
   - For Google Cloud Monitoring built-in metrics they are usually
     prefixed by the name of the service, e.g.
     `run.googleapis.com/request_count` from Cloud Run.
     - Tip: If you are unsure what metrics are available: use Metrics
       Editor, select a "resource type", the UI will give you a list of
       metrics specific to that resource type.
3. `filter metric.result == "OK"`: a filter on the result. You can add
   multiple predicates here: `filter foo && bar`.  Auto completion
   should help you explore available fields to filter on.
4. `[metric.realm]`: a group by directive. Note this is a concise form,
   you can also write `group_by [metric.realm]` if that makes it more
   readable. Auto completion should help you explore available fields to
   group by.
5. `sum`: Aggregation function, necessary if you have a group by in your
   MQL. See [MQL aggregation][] for other aggregation functions.


[Google Cloud Console]: https://console.cloud.google.com
[Logging query language]: https://cloud.google.com/logging/docs/view/logging-query-language
[MQL aggregation]: https://cloud.google.com/monitoring/mql/reference#aggr-function-group
[MQL examples]: https://cloud.google.com/monitoring/mql/examples
[MQL reference]: https://cloud.google.com/monitoring/mql/reference
[Using Metrics Explorer]: https://cloud.google.com/monitoring/charts/metrics-explorer#find-me
[Using the Logs Explorer]: https://cloud.google.com/logging/docs/view/logs-viewer-interface
[Using the Query Editor]: https://cloud.google.com/monitoring/mql/query-editor
