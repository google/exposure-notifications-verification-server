module github.com/google/exposure-notifications-verification-server

go 1.15

require (
	cloud.google.com/go v0.74.0
	cloud.google.com/go/firestore v1.4.0 // indirect
	contrib.go.opencensus.io/exporter/ocagent v0.7.1-0.20200615190824-f8c219d2d895 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.2.1-0.20200609204449-6bcf6f8577f0 // indirect
	contrib.go.opencensus.io/integrations/ocsql v0.1.7
	firebase.google.com/go v3.13.0+incompatible
	github.com/Azure/azure-sdk-for-go v49.2.0+incompatible // indirect
	github.com/DataDog/datadog-go v3.7.1+incompatible // indirect
	github.com/Jeffail/gabs/v2 v2.5.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/aliyun/alibaba-cloud-sdk-go v1.61.213 // indirect
	github.com/armon/go-proxyproto v0.0.0-20200108142055-f0b8253b1507 // indirect
	github.com/asaskevich/govalidator v0.0.0-20200108200545-475eaeb16496 // indirect
	github.com/aws/aws-sdk-go v1.36.16 // indirect
	github.com/chromedp/cdproto v0.0.0-20201204063249-be40c824ad18
	github.com/chromedp/chromedp v0.5.4
	github.com/circonus-labs/circonusllhist v0.1.4 // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gobwas/httphead v0.1.0 // indirect
	github.com/golang-migrate/migrate/v4 v4.14.1
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9
	github.com/google/exposure-notifications-server v0.21.1-0.20210203231836-b0cfb8b1fad8
	github.com/google/go-cmp v0.5.4
	github.com/google/uuid v1.1.2
	github.com/gorilla/csrf v1.7.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/gorilla/schema v1.2.0
	github.com/gorilla/securecookie v1.1.1
	github.com/gorilla/sessions v1.2.1
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/hashicorp/go-memdb v1.2.1 // indirect
	github.com/hashicorp/go-msgpack v1.1.5 // indirect
	github.com/hashicorp/go-multierror v1.1.0
	github.com/hashicorp/go-plugin v1.3.0 // indirect
	github.com/hashicorp/hcl/v2 v2.8.1
	github.com/hashicorp/yamux v0.0.0-20190923154419-df201c70410d // indirect
	github.com/jefferai/jsonx v1.0.1 // indirect
	github.com/jinzhu/gorm v1.9.16
	github.com/keybase/go-crypto v0.0.0-20200123153347-de78d2cb44f4 // indirect
	github.com/leonelquinteros/gotext v1.4.0
	github.com/lib/pq v1.9.0
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/microcosm-cc/bluemonday v1.0.4
	github.com/mikehelmick/go-chaff v0.4.1
	github.com/mitchellh/go-testing-interface v1.14.1 // indirect
	github.com/oklog/run v1.1.0 // indirect
	github.com/onsi/ginkgo v1.13.0 // indirect
	github.com/opencensus-integrations/redigo v2.0.1+incompatible
	github.com/oracle/oci-go-sdk v19.3.0+incompatible // indirect
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/posener/complete v1.2.3 // indirect
	github.com/rakutentech/jwk-go v1.0.1
	github.com/russross/blackfriday/v2 v2.1.0
	github.com/sethvargo/go-envconfig v0.3.2
	github.com/sethvargo/go-limiter v0.6.0
	github.com/sethvargo/go-password v0.2.0
	github.com/sethvargo/go-redisstore v0.3.0-opencensus
	github.com/sethvargo/go-retry v0.1.0
	github.com/sethvargo/go-signalcontext v0.1.0
	github.com/sethvargo/zapw v0.1.0
	github.com/smartystreets/assertions v1.0.0 // indirect
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/timakin/bodyclose v0.0.0-20200424151742-cb6215831a94
	github.com/tv42/httpunix v0.0.0-20191220191345-2ba4b9c3382c // indirect
	github.com/unrolled/secure v1.0.8
	go.opencensus.io v0.22.5
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/net v0.0.0-20201224014010-6772e930b67b // indirect
	golang.org/x/sys v0.0.0-20201223074533-0d417f636930 // indirect
	golang.org/x/text v0.3.5
	golang.org/x/tools v0.0.0-20201228204837-84d76fe3206d
	google.golang.org/api v0.36.0
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d
	gopkg.in/gormigrate.v1 v1.6.0
	gopkg.in/ini.v1 v1.56.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20200615113413-eeeca48fe776 // indirect
	honnef.co/go/tools v0.1.0
	k8s.io/api v0.18.7-rc.0 // indirect
	sigs.k8s.io/structured-merge-diff/v3 v3.0.1-0.20200706213357-43c19bbb7fba // indirect
)
