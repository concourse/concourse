package topgun_test

import (
	"database/sql"
	"testing"

	gclient "code.cloudfoundry.org/garden/client"
	"code.cloudfoundry.org/lager/lagertest"
	sq "github.com/Masterminds/squirrel"
	bclient "github.com/concourse/baggageclaim/client"
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

func TestTOPGUN(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Core and Runtime Suite")
}

