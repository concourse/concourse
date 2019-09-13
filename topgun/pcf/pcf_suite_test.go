package topgun_test

import (
	"regexp"
	"database/sql"
	"testing"

	sq "github.com/Masterminds/squirrel"
	bclient "github.com/concourse/baggageclaim/client"
	gclient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/lager/lagertest"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/concourse/concourse/topgun"
	. "github.com/concourse/concourse/topgun/common"

)


var (
	deploymentNamePrefix string

	fly                       = Fly{}
	deploymentName, flyTarget string
	instances                 map[string][]BoshInstance
	jobInstances              map[string][]BoshInstance

	dbInstance *BoshInstance
	dbConn     *sql.DB

	webInstance    *BoshInstance
	atcExternalURL string
	atcUsername    string
	atcPassword    string

	workerGardenClient       gclient.Client
	workerBaggageclaimClient bclient.Client

	concourseReleaseVersion, bpmReleaseVersion, postgresReleaseVersion string
	vaultReleaseVersion, credhubReleaseVersion                         string
	stemcellVersion                                                    string
	backupAndRestoreReleaseVersion                                     string

	pipelineName string

	logger *lagertest.TestLogger

	tmp string
)

var psql = sq.StatementBuilder.PlaceholderFormat(sq.Dollar)

var instanceRow = regexp.MustCompile(`^([^/]+)/([^\s]+)\s+-\s+(\w+)\s+z1\s+([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+)\s+([^\s]+)\s*$`)
var jobRow = regexp.MustCompile(`^([^\s]+)\s+(\w+)\s+(\w+)\s+-\s+-\s+-\s*$`)

func TestTOPGUN(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TOPGUN Suite")
}
