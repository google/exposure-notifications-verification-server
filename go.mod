module github.com/google/exposure-notifications-verification-server

go 1.16

require (
	cloud.google.com/go v0.87.0
	cloud.google.com/go/firestore v1.5.0 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.7.1-0.20200615190824-f8c219d2d895 // indirect
	contrib.go.opencensus.io/integrations/ocsql v0.1.7
	firebase.google.com/go v3.13.0+incompatible
	github.com/NYTimes/gziphandler v1.1.1
	github.com/chromedp/cdproto v0.0.0-20210713064928-7d28b402946a
	github.com/chromedp/chromedp v0.7.4
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/structs v1.1.0
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/go-test/deep v1.0.7 // indirect
	github.com/golang-jwt/jwt v3.2.1+incompatible
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9
	github.com/google/exposure-notifications-server v0.33.0
	github.com/google/go-cmp v0.5.6
	github.com/google/uuid v1.3.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/hcl v1.0.1-vault-3 // indirect
	github.com/jinzhu/gorm v1.9.16
	github.com/kelseyhightower/run v0.0.17
	github.com/leonelquinteros/gotext v1.5.0
	github.com/lib/pq v1.10.2
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/microcosm-cc/bluemonday v1.0.15
	github.com/mikehelmick/go-chaff v0.5.0
	github.com/nyaruka/phonenumbers v1.0.70
	github.com/opencensus-integrations/redigo v2.0.1+incompatible
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/rakutentech/jwk-go v1.0.1
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/sethvargo/go-envconfig v0.3.5
	github.com/sethvargo/go-limiter v0.6.0
	github.com/sethvargo/go-password v0.2.0
	github.com/sethvargo/go-redisstore v0.3.0-opencensus
	github.com/sethvargo/go-retry v0.1.0
	github.com/sethvargo/zapw v0.1.0
	github.com/unrolled/secure v1.0.9
	go.opencensus.io v0.23.0
	go.uber.org/zap v1.18.1
	golang.org/x/net v0.0.0-20210716203947-853a461950ff
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/text v0.3.6
	golang.org/x/tools v0.1.5
	google.golang.org/api v0.50.0
	google.golang.org/genproto v0.0.0-20210719143636-1d5a45f8e492
	gopkg.in/gormigrate.v1 v1.6.0
)
