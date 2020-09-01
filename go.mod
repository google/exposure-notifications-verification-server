module github.com/google/exposure-notifications-verification-server

go 1.14

require (
	cloud.google.com/go v0.65.0
	cloud.google.com/go/firestore v1.3.0 // indirect
	firebase.google.com/go v3.13.0+incompatible
	github.com/client9/misspell v0.3.4
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/google/exposure-notifications-server v0.6.1-0.20200901200240-c75b1e7ab942
	github.com/google/go-cmp v0.5.2
	github.com/gorilla/csrf v1.7.0
	github.com/gorilla/handlers v1.5.0
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/gorilla/sessions v1.2.1
	github.com/jinzhu/gorm v1.9.16
	github.com/mikehelmick/go-chaff v0.3.0
	github.com/opencensus-integrations/redigo v2.0.1+incompatible
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/rakutentech/jwk-go v1.0.1
	github.com/sethvargo/go-envconfig v0.3.1
	github.com/sethvargo/go-limiter v0.4.1
	github.com/sethvargo/go-redisstore v0.1.2-opencensus
	github.com/sethvargo/go-retry v0.1.0
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/unrolled/secure v1.0.8
	go.opencensus.io v0.22.4
	go.uber.org/zap v1.15.0
	golang.org/x/time v0.0.0-20200630173020-3af7569d3a1e
	golang.org/x/tools v0.0.0-20200831203904-5a2aa26beb65
	google.golang.org/genproto v0.0.0-20200831141814-d751682dd103
	gopkg.in/gormigrate.v1 v1.6.0
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	honnef.co/go/tools v0.0.1-2020.1.5
)
