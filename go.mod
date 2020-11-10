module github.com/google/exposure-notifications-verification-server

go 1.15

replace github.com/jackc/puddle => github.com/jeremyfaller/puddle v1.1.2-0.20200821025810-91d0159cc97a

replace golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6

require (
	cloud.google.com/go v0.71.0
	cloud.google.com/go/firestore v1.3.0 // indirect
	contrib.go.opencensus.io/integrations/ocsql v0.1.6
	firebase.google.com/go v3.13.0+incompatible
	github.com/Azure/azure-sdk-for-go v48.1.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.11 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.3 // indirect
	github.com/Microsoft/go-winio v0.4.15 // indirect
	github.com/aws/aws-sdk-go v1.35.24 // indirect
	github.com/chromedp/cdproto v0.0.0-20201009231348-1c6a710e77de
	github.com/chromedp/chromedp v0.5.3
	github.com/client9/misspell v0.3.4
	github.com/containerd/continuity v0.0.0-20200928162600-f2cc35102c2a // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9
	github.com/google/exposure-notifications-server v0.16.0
	github.com/google/go-cmp v0.5.2
	github.com/google/uuid v1.1.2
	github.com/gorilla/csrf v1.7.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-retryablehttp v0.6.8 // indirect
	github.com/jinzhu/gorm v1.9.16
	github.com/jinzhu/now v1.1.1 // indirect
	github.com/lib/pq v1.8.0
	github.com/mattn/go-sqlite3 v2.0.1+incompatible // indirect
	github.com/microcosm-cc/bluemonday v1.0.4
	github.com/mikehelmick/go-chaff v0.3.0
	github.com/opencensus-integrations/redigo v2.0.1+incompatible
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/pierrec/lz4 v2.6.0+incompatible // indirect
	github.com/prometheus/client_golang v1.8.0 // indirect
	github.com/prometheus/common v0.15.0 // indirect
	github.com/rakutentech/jwk-go v1.0.1
	github.com/russross/blackfriday/v2 v2.0.1
	github.com/sethvargo/go-envconfig v0.3.2
	github.com/sethvargo/go-limiter v0.6.0
	github.com/sethvargo/go-password v0.2.0
	github.com/sethvargo/go-redisstore v0.3.0-opencensus
	github.com/sethvargo/go-retry v0.1.0
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/unrolled/secure v1.0.8
	go.opencensus.io v0.22.5
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897 // indirect
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b // indirect
	golang.org/x/oauth2 v0.0.0-20201109201403-9fd604954f58 // indirect
	golang.org/x/sys v0.0.0-20201109165425-215b40eba54c // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	golang.org/x/tools v0.0.0-20201110124207-079ba7bd75cd
	google.golang.org/api v0.35.0
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201110150050-8816d57aaa9a
	google.golang.org/grpc v1.33.2 // indirect
	gopkg.in/gormigrate.v1 v1.6.0
	honnef.co/go/tools v0.0.1-2020.1.6
)
