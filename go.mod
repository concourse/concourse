module github.com/concourse/concourse

require (
	cloud.google.com/go v0.28.0 // indirect
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c
	code.cloudfoundry.org/credhub-cli v0.0.0-20180814203433-814bc1b711fe
	code.cloudfoundry.org/garden v0.0.0-20180820151144-7999b305fe99
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/localip v0.0.0-20170223024724-b88ad0dea95c
	code.cloudfoundry.org/urljoiner v0.0.0-20170223060717-5cabba6c0a50
	github.com/DataDog/datadog-go v0.0.0-20180822151419-281ae9f2d895
	github.com/Masterminds/squirrel v0.0.0-20180802154824-cebd809c54c4
	github.com/NYTimes/gziphandler v1.0.1
	github.com/PuerkitoBio/purell v1.1.0 // indirect
	github.com/PuerkitoBio/urlesc v0.0.0-20170810143723-de5bf2ad4578 // indirect
	github.com/The-Cloud-Source/goryman v0.0.0-20150410173800-c22b6e4a7ac1
	github.com/aryann/difflib v0.0.0-20170710044230-e206f873d14a
	github.com/aws/aws-sdk-go v1.15.64
	github.com/bmatcuk/doublestar v1.1.1 // indirect
	github.com/caarlos0/env v3.5.0+incompatible
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/charlievieth/fs v0.0.0-20170613215519-7dc373669fa1 // indirect
	github.com/cloudfoundry/bosh-cli v5.4.0+incompatible
	github.com/cloudfoundry/bosh-utils v0.0.0-20180919212956-15c556314b68 // indirect
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/socks5-proxy v0.0.0-20180530211953-3659db090cb2 // indirect
	github.com/concourse/baggageclaim v1.3.3
	github.com/concourse/dex v0.0.0-20181120155244-024cbea7e753
	github.com/concourse/flag v0.0.0-20180907155614-cb47f24fff1c
	github.com/concourse/go-archive v1.0.0
	github.com/concourse/retryhttp v0.0.0-20181126170240-7ab5e29e634f
	github.com/coreos/go-oidc v0.0.0-20170307191026-be73733bb8cc
	github.com/cppforlife/go-patch v0.0.0-20171006213518-250da0e0e68c // indirect
	github.com/cppforlife/go-semi-semantic v0.0.0-20160921010311-576b6af77ae4
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/emicklei/go-restful v2.8.0+incompatible // indirect
	github.com/fatih/color v1.7.0
	github.com/felixge/httpsnoop v1.0.0
	github.com/felixge/tcpkeepalive v0.0.0-20160804073959-5bb0b2dea91e
	github.com/go-openapi/jsonpointer v0.0.0-20180825180259-52eb3d4b47c6 // indirect
	github.com/go-openapi/jsonreference v0.0.0-20180825180305-1c6a3fa339f2 // indirect
	github.com/go-openapi/spec v0.0.0-20180825180323-f1468acb3b29 // indirect
	github.com/go-openapi/swag v0.0.0-20180908172849-dd0dad036e67 // indirect
	github.com/gobuffalo/packr v1.13.7
	github.com/google/gofuzz v0.0.0-20170612174753-24818f796faf // indirect
	github.com/google/jsonapi v0.0.0-20180618021926-5d047c6bc66b
	github.com/googleapis/gnostic v0.2.0 // indirect
	github.com/gorilla/websocket v1.4.0
	github.com/gregjones/httpcache v0.0.0-20180305231024-9cad4c3443a7 // indirect
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/vault v0.11.5
	github.com/howeyc/gopass v0.0.0-20170109162249-bf9dde6d0d2c // indirect
	github.com/inconshreveable/go-update v0.0.0-20160112193335-8152e7eb6ccf
	github.com/influxdata/influxdb v1.7.2
	github.com/influxdata/platform v0.0.0-20190110054358-fbbe20953ffd // indirect
	github.com/jessevdk/go-flags v1.4.0
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/juju/ratelimit v1.0.1 // indirect
	github.com/kr/pty v1.1.3
	github.com/krishicks/yaml-patch v0.0.10
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/lib/pq v1.0.0
	github.com/mailru/easyjson v0.0.0-20180823135443-60711f1a8329 // indirect
	github.com/mattn/go-colorable v0.0.9
	github.com/mattn/go-isatty v0.0.4
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/miekg/dns v1.1.1
	github.com/mitchellh/mapstructure v1.1.2
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.7.0
	github.com/onsi/gomega v1.4.3
	github.com/papertrail/remote_syslog2 v0.0.0-20170912230402-5bae4a1ac1c2
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/peterhellberg/link v1.0.0
	github.com/pkg/term v0.0.0-20180730021639-bffc007b7fd5
	github.com/prometheus/client_golang v0.9.0
	github.com/sirupsen/logrus v1.3.0
	github.com/skratchdot/open-golang v0.0.0-20160302144031-75fb7ed4208c
	github.com/square/certstrap v1.1.1
	github.com/tedsuo/ifrit v0.0.0-20180802180643-bea94bb476cc
	github.com/tedsuo/rata v1.0.1-0.20170830210128-07d200713958
	github.com/vito/go-interact v0.0.0-20171111012221-fa338ed9e9ec
	github.com/vito/go-sse v0.0.0-20160212001227-fd69d275caac
	github.com/vito/houdini v0.0.0-20170630141751-8dda540e3245
	github.com/vito/twentythousandtonnesofcrudeoil v0.0.0-20180305154709-3b21ad808fcb
	golang.org/x/crypto v0.0.0-20181203042331-505ab145d0a9
	golang.org/x/net v0.0.0-20181217023233-e147a9138326 // indirect
	golang.org/x/oauth2 v0.0.0-20181017192945-9dcd33a902f4
	golang.org/x/sync v0.0.0-20181108010431-42b317875d0f // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.27
	gopkg.in/square/go-jose.v2 v2.1.8
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20171027084545-218912509d74
	k8s.io/apimachinery v0.0.0-20171027084411-18a564baac72
	k8s.io/client-go v2.0.0-alpha.0.0.20171101191150-72e1c2a1ef30+incompatible
	k8s.io/kube-openapi v0.0.0-20180731170545-e3762e86a74c // indirect
)

replace github.com/hashicorp/go-msgpack => github.com/evandigby/go-msgpack v0.0.0-20180728010727-b3f48f4eda2a
