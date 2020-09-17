module github.com/google/exposure-notifications-verification-server

go 1.15

require (
	cloud.google.com/go v0.65.0
	cloud.google.com/go/firestore v1.3.0 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.7.1-0.20200615190824-f8c219d2d895 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.2.1-0.20200609204449-6bcf6f8577f0 // indirect
	contrib.go.opencensus.io/integrations/ocsql v0.1.6
	firebase.google.com/go v3.13.0+incompatible
	github.com/Microsoft/go-winio v0.4.15-0.20190919025122-fc70bd9a86b5 // indirect
	github.com/asaskevich/govalidator v0.0.0-20200108200545-475eaeb16496 // indirect
	github.com/client9/misspell v0.3.4
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/frankban/quicktest v1.8.1 // indirect
	github.com/google/exposure-notifications-server v0.8.0
	github.com/google/go-cmp v0.5.2
	github.com/gorilla/csrf v1.7.0
	github.com/gorilla/handlers v1.5.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/gorilla/sessions v1.2.1
	github.com/jinzhu/gorm v1.9.16
	github.com/jinzhu/now v1.1.1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/lib/pq v1.8.0
	github.com/mattn/go-sqlite3 v2.0.1+incompatible // indirect
	github.com/mikehelmick/go-chaff v0.3.0
	github.com/opencensus-integrations/redigo v2.0.1+incompatible
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/rakutentech/jwk-go v1.0.1
	github.com/sethvargo/go-envconfig v0.3.1
	github.com/sethvargo/go-limiter v0.5.1
	github.com/sethvargo/go-password v0.2.0
	github.com/sethvargo/go-redisstore v0.2.0-opencensus
	github.com/sethvargo/go-retry v0.1.0
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/unrolled/secure v1.0.8
	go.opencensus.io v0.22.4
	go.uber.org/zap v1.16.0
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	golang.org/x/tools v0.0.0-20200908191908-acefd226e2cc
	google.golang.org/genproto v0.0.0-20200904004341-0bd0a958aa1d
	gopkg.in/gormigrate.v1 v1.6.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	honnef.co/go/tools v0.0.1-2020.1.5
	k8s.io/api v0.18.7-rc.0 // indirect
	sigs.k8s.io/structured-merge-diff/v3 v3.0.1-0.20200706213357-43c19bbb7fba // indirect
)
