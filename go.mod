module github.com/concourse/concourse

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c
	code.cloudfoundry.org/credhub-cli v0.0.0-20190415201820-e3951663d25c
	code.cloudfoundry.org/garden v0.0.0-20181108172608-62470dc86365
	code.cloudfoundry.org/lager v2.0.0+incompatible
	code.cloudfoundry.org/localip v0.0.0-20170223024724-b88ad0dea95c
	code.cloudfoundry.org/urljoiner v0.0.0-20170223060717-5cabba6c0a50
	github.com/DataDog/datadog-go v3.7.2+incompatible
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.11.0
	github.com/Masterminds/squirrel v1.5.0
	github.com/Microsoft/go-winio v0.4.17-0.20210211115548-6eac466e5fa3 // indirect
	github.com/Microsoft/hcsshim v0.8.7 // indirect
	github.com/NYTimes/gziphandler v1.1.1
	github.com/aryann/difflib v0.0.0-20170710044230-e206f873d14a
	github.com/aws/aws-sdk-go v1.38.9
	github.com/caarlos0/env v3.5.0+incompatible
	github.com/cenkalti/backoff v2.2.1+incompatible
	github.com/cloudfoundry/go-socks5 v0.0.0-20180221174514-54f73bdb8a8e // indirect
	github.com/cloudfoundry/socks5-proxy v0.0.0-20180530211953-3659db090cb2 // indirect
	github.com/concourse/baggageclaim v1.11.0
	github.com/concourse/dex v0.7.0
	github.com/concourse/flag v1.1.0
	github.com/concourse/go-archive v1.0.1
	github.com/concourse/retryhttp v1.1.0
	github.com/containerd/cgroups v0.0.0-20210114181951-8a68de567b68 // indirect
	github.com/containerd/console v1.0.1 // indirect
	github.com/containerd/containerd v1.4.4
	github.com/containerd/continuity v0.0.0-20201208142359-180525291bb7 // indirect
	github.com/containerd/fifo v0.0.0-20201026212402-0724c46b320c // indirect
	github.com/containerd/go-cni v1.0.1
	github.com/containerd/go-runc v0.0.0-20200220073739-7016d3ce2328 // indirect
	github.com/containerd/ttrpc v1.0.2 // indirect
	github.com/containerd/typeurl v1.0.1
	github.com/coreos/go-iptables v0.5.0
	github.com/cppforlife/go-semi-semantic v0.0.0-20160921010311-576b6af77ae4
	github.com/cyberark/conjur-api-go v0.7.1
	github.com/docker/go-events v0.0.0-20190806004212-e31b211e4f1c // indirect
	github.com/fatih/color v1.10.0
	github.com/felixge/httpsnoop v1.0.1
	github.com/gobwas/glob v0.2.3
	github.com/goccy/go-yaml v1.8.9
	github.com/gogo/googleapis v1.4.0 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang/groupcache v0.0.0-20200121045136-8c9f03a8e57e
	github.com/google/jsonapi v0.0.0-20180618021926-5d047c6bc66b
	github.com/gorilla/websocket v1.4.2
	github.com/hashicorp/go-multierror v1.1.1
	github.com/hashicorp/go-rootcerts v1.0.2
	github.com/hashicorp/vault/api v1.0.5-0.20191108163347-bdd38fca2cff
	github.com/hashicorp/vault/sdk v0.1.14-0.20191112033314-390e96e22eb2 // indirect
	github.com/honeycombio/opentelemetry-exporter-go v0.11.0
	github.com/imdario/mergo v0.3.12
	github.com/inconshreveable/go-update v0.0.0-20160112193335-8152e7eb6ccf
	github.com/influxdata/influxdb1-client v0.0.0-20191209144304-8bf82d3c094d
	github.com/jessevdk/go-flags v1.4.1-0.20200711081900-c17162fe8fd7
	github.com/klauspost/compress v1.11.13
	github.com/kr/pty v1.1.8
	github.com/krishicks/yaml-patch v0.0.10
	github.com/lib/pq v1.10.0
	github.com/mattn/go-colorable v0.1.8
	github.com/mattn/go-isatty v0.0.12
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.4.1
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b
	github.com/miekg/dns v1.1.41
	github.com/mitchellh/mapstructure v1.4.1
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.16.1
	github.com/onsi/gomega v1.11.0
	github.com/opencontainers/runc v1.0.0-rc90 // indirect
	github.com/opencontainers/runtime-spec v1.0.2
	github.com/opencontainers/selinux v1.8.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/peterhellberg/link v1.1.0
	github.com/pkg/errors v0.9.1
	github.com/pkg/term v0.0.0-20190109203006-aa71e9d9e942
	github.com/prometheus/client_golang v1.10.0
	github.com/racksec/srslog v0.0.0-20180709174129-a4725f04ec91
	github.com/sirupsen/logrus v1.8.1
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966
	github.com/square/certstrap v1.1.1
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tedsuo/ifrit v0.0.0-20180802180643-bea94bb476cc
	github.com/tedsuo/rata v1.0.1-0.20170830210128-07d200713958
	github.com/vbauerster/mpb/v4 v4.6.1-0.20190319154207-3a6acfe12ac6
	github.com/vito/go-interact v0.0.0-20171111012221-fa338ed9e9ec
	github.com/vito/go-sse v0.0.0-20160212001227-fd69d275caac
	github.com/vito/houdini v1.1.1
	github.com/vito/twentythousandtonnesofcrudeoil v0.0.0-20180305154709-3b21ad808fcb
	go.etcd.io/bbolt v1.3.5 // indirect
	go.opentelemetry.io/collector v0.24.0
	go.opentelemetry.io/otel v0.11.0
	go.opentelemetry.io/otel/exporters/otlp v0.11.0
	go.opentelemetry.io/otel/exporters/trace/jaeger v0.11.0
	go.opentelemetry.io/otel/sdk v0.11.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/oauth2 v0.0.0-20210210192628-66670185b0cd
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/time v0.0.0-20201208040808-7e3f01d25324
	google.golang.org/grpc v1.37.0
	gopkg.in/square/go-jose.v2 v2.5.1
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.20.5
	k8s.io/apimachinery v0.21.0
	k8s.io/client-go v0.20.5
	sigs.k8s.io/yaml v1.2.0
)

go 1.16

replace github.com/docker/distribution v2.7.1+incompatible => github.com/docker/distribution v2.7.1-0.20190205005809-0d3efadf0154+incompatible
