# StackdriverExportFailed

OpenCensus failed to export metrics to Stackdriver.

This means all other alerts we have configured won't work as the alert depend
on the export metrics.

NOTE: metric export may spontanously fail. If the failure rate is low it's
likely the threshold is too sensitive.
