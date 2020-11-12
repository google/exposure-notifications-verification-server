# E2ETestErrorRatioHigh

The e2e test is failing.

Use the following MQL query to figure out the step at which the e2e test
fail, then check the e2e-runner service log as well as the source code
(pkg/clients/e2e.go):

```
generic_task ::
custom.googleapis.com/opencensus/en-verification-server/e2e/request_count
| (metric.result != 'OK')
| align rate(1m)
| every 1m
| [metric.step]
```
