<!-- TOC depthFrom:2 -->

- [Services](#services)
  - [Admin API](#admin-api)
  - [API Server](#api-server)
  - [App Sync Server](#app-sync-server)
  - [Cleanup Server](#cleanup-server)
  - [End-to-end Runner Server](#end-to-end-runner-server)
  - [ENX Redirect Server](#enx-redirect-server)
  - [Modeler Server](#modeler-server)
  - [Rotation Server](#rotation-server)
  - [Server](#server)
  - [Stats Puller Server](#stats-puller-server)
- [Dependencies](#dependencies)
  - [PostgreSQL](#postgresql)
  - [Redis](#redis)
  - [Identity Platform](#identity-platform)

<!-- /TOC -->

# Architecture

![Verification Flow](https://developers.google.com/android/exposure-notifications/images/verification-flow.svg)

## Services

This "server" is actually a collection of small services which are designed to
be serverless and scale independently. Note that the system is not considered a
microservices architecture, since it does not require components like service
discovery to function.


### Admin API

- Name: `adminapi`
- Path: `./cmd/adminapi`
- Public: yes

The Admin API is a JSON API server for issuing codes and gathering statistics.
It provides an automated way for PHAs to build their own [UI server](#ui-server)
or integrate more closely with an existing ERP system.

### API Server

- Name: `apiserver`
- Path: `./cmd/apiserver`
- Public: yes

The API server is a JSON API server with which mobile devices ("apps") interact
to verify codes and exchange certificates.


### App Sync Server

- Name: `appsync`
- Path: `./cmd/appsync`
- Public: no

The app sync server is an internal service that synchronizes data from public
app stores into the system. It is invoked periodically via a distributed cron.


### Cleanup Server

- Name: `cleanup`
- Path: `./cmd/cleanup`
- Public: no

The cleanup server is an internal service that deletes or purges data from the
system. For the most up-to-date information on what is purged and with what TTL,
please see the service's configuration. Examples of data that are cleaned up
include stale users, old audit log entires, or expired certificates. It is
invoked periodically via a distributed cron.


### End-to-end Runner Server

- Name: `e2e-runner`
- Path: `./cmd/e2e-runner`
- Public: no

The end-to-end runner server is an internal service that simulates a full client
code, certificate, and TEK exchange. It is used for continuous integration and
smoke testing, and requires a key server as configuration. It is invoked
periodically via a distributed cron.


### ENX Redirect Server

- Name: `enx-redirect`
- Path: `./cmd/enx-redirect`
- Public: yes

The ENX redirect server serves Apple/Google `.well-known` association files for
app links to allow PHAs to gracefully redirect users to an appropriate app store
in the event their application is not installed.


### Modeler Server

- Name: `modeler`
- Path: `./cmd/modeler`
- Public: no

The modeler server is an internal service that uses a PHA's historical data to
build a predictive model of future code issuances. This model is used as part of
the optional Abuse Prevention feature. It is invoked periodically via a
distributed cron.


### Rotation Server

- Name: `rotation`
- Path: `./cmd/rotation`
- Public: no

The rotation server is an internal service that creates new versions of various
keys, certificates, and secrets. It is invoked periodically via a distributed
cron.


### Server

- Name: `server`
- Path: `./cmd/server`
- Public: yes

The server is the main web interface with which case workers, PHAs, and
system administrators interface with the system. It can create codes, retrieve
and visualize statistics, and serves audit logging and other important system
information.


### Stats Puller Server

- Name: `stats-puller`
- Path: `./cmd/stats-puller`
- Public: no

The stats-puller server is an internal service that pulls data from a key
server. It is invoked periodically via a distributed cron.


## Dependencies

### PostgreSQL

The system uses PostgreSQL to maintain shared state and distributed locking.

### Redis

The system uses Redis for caching and distributed rate limiting.

### Identity Platform

The system uses Cloud Identity Platform for managed authentication and sign-in.
