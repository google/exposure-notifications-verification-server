# Production

This page includes helpful tips for configuring things in production:

## Observability (tracing and metrics)

The observability component is responsible for metrics. The following
configurations are available:

| Name                    | `OBSERVABILITY_EXPORTER` value  | Description
| ----------------------- | ------------------------------- | -----------
| OpenCensus Agent        | `OCAGENT`                       | Use OpenCensus.
| Stackdriver\*           | `STACKDRIVER`                   | Use Stackdriver.
