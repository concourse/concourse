package postgresrunner

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"code.cloudfoundry.org/lager/lagertest"

	"github.com/concourse/atc/db"
	"github.com/concourse/atc/db/encryption"
	"github.com/concourse/atc/db/migration"
	"github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
	"github.com/tedsuo/ifrit/ginkgomon"
)

type Runner struct {
	Port int
}

func (runner Runner) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	defer ginkgo.GinkgoRecover()

	pgBase := filepath.Join(os.TempDir(), "concourse-pg-runner")

	err := os.MkdirAll(pgBase, 0755)
	Expect(err).NotTo(HaveOccurred())

	tmpdir, err := ioutil.TempDir(pgBase, "postgres")
	Expect(err).NotTo(HaveOccurred())

	currentUser, err := user.Current()
	Expect(err).NotTo(HaveOccurred())

	initdbPath, err := exec.LookPath("initdb")
	Expect(err).NotTo(HaveOccurred())

	postgresPath, err := exec.LookPath("postgres")
	Expect(err).NotTo(HaveOccurred())

	initCmd := exec.Command(initdbPath, "-U", "postgres", "-D", tmpdir, "-E", "UTF8", "--no-local")
	startCmd := exec.Command(postgresPath, "-D", tmpdir, "-h", "127.0.0.1", "-p", strconv.Itoa(runner.Port))

	if currentUser.Uid == "0" {
		pgUser, err := user.Lookup("postgres")
		Expect(err).NotTo(HaveOccurred())

		var uid, gid uint32
		_, err = fmt.Sscanf(pgUser.Uid, "%d", &uid)
		Expect(err).NotTo(HaveOccurred())

		_, err = fmt.Sscanf(pgUser.Gid, "%d", &gid)
		Expect(err).NotTo(HaveOccurred())

		err = os.Chown(tmpdir, int(uid), int(gid))
		Expect(err).NotTo(HaveOccurred())

		credential := &syscall.Credential{Uid: uid, Gid: gid}

		initCmd.SysProcAttr = &syscall.SysProcAttr{}
		initCmd.SysProcAttr.Credential = credential

		startCmd.SysProcAttr = &syscall.SysProcAttr{}
		startCmd.SysProcAttr.Credential = credential
	}

	session, err := gexec.Start(
		initCmd,
		gexec.NewPrefixedWriter("[o][initdb] ", ginkgo.GinkgoWriter),
		gexec.NewPrefixedWriter("[e][initdb] ", ginkgo.GinkgoWriter),
	)
	Expect(err).NotTo(HaveOccurred())

	<-session.Exited

	Expect(session).To(gexec.Exit(0))

	ginkgoRunner := &ginkgomon.Runner{
		Name:          "postgres",
		Command:       startCmd,
		AnsiColorCode: "90m",
		StartCheck:    "database system is ready to accept connections",
		Cleanup: func() {
			os.RemoveAll(tmpdir)
		},
	}

	return ginkgoRunner.Run(signals, ready)
}

func (runner *Runner) MigrateToVersion(version int) {
	err := migration.NewOpenHelper(
		"postgres",
		runner.DataSourceName(),
		nil,
		encryption.NewNoEncryption(),
	).MigrateToVersion(version)
	Expect(err).NotTo(HaveOccurred())
}

func (runner *Runner) TryOpenDBAtVersion(version int) (*sql.DB, error) {
	dbConn, err := migration.NewOpenHelper(
		"postgres",
		runner.DataSourceName(),
		nil,
		encryption.NewNoEncryption(),
	).OpenAtVersion(version)

	if err != nil {
		return nil, err
	}

	// only allow one connection so that we can detect any code paths that
	// require more than one, which will deadlock if it's at the limit
	dbConn.SetMaxOpenConns(1)

	return dbConn, nil
}

func (runner *Runner) OpenDBAtVersion(version int) *sql.DB {
	dbConn, err := runner.TryOpenDBAtVersion(version)
	Expect(err).NotTo(HaveOccurred())
	return dbConn
}

func (runner *Runner) OpenDB() *sql.DB {
	dbConn, err := migration.NewOpenHelper(
		"postgres",
		runner.DataSourceName(),
		nil,
		encryption.NewNoEncryption(),
	).Open()
	Expect(err).NotTo(HaveOccurred())

	// only allow one connection so that we can detect any code paths that
	// require more than one, which will deadlock if it's at the limit
	dbConn.SetMaxOpenConns(1)

	return dbConn
}

func (runner *Runner) OpenConn() db.Conn {
	dbConn, err := db.Open(
		lagertest.NewTestLogger("postgres-runner"),
		"postgres",
		runner.DataSourceName(),
		nil,
		nil,
		"postgresrunner",
		nil,
	)
	Expect(err).NotTo(HaveOccurred())

	// only allow one connection so that we can detect any code paths that
	// require more than one, which will deadlock if it's at the limit
	dbConn.SetMaxOpenConns(1)

	return dbConn
}

func (runner *Runner) OpenSingleton() *sql.DB {
	dbConn, err := sql.Open("postgres", runner.DataSourceName())
	Expect(err).NotTo(HaveOccurred())

	// only allow one connection, period. this matches production code use case,
	// as this is used for advisory locks.
	dbConn.SetMaxIdleConns(1)
	dbConn.SetMaxOpenConns(1)
	dbConn.SetConnMaxLifetime(0)

	return dbConn
}

func (runner *Runner) DataSourceName() string {
	return fmt.Sprintf("user=postgres dbname=testdb sslmode=disable port=%d", runner.Port)
}

func (runner *Runner) CreateTestDB() {
	createdb := exec.Command("createdb", "-U", "postgres", "-p", strconv.Itoa(runner.Port), "testdb")

	createS, err := gexec.Start(createdb, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-createS.Exited

	if createS.ExitCode() != 0 {
		runner.DropTestDB()

		createdb := exec.Command("createdb", "-U", "postgres", "-p", strconv.Itoa(runner.Port), "testdb")
		createS, err = gexec.Start(createdb, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
	}

	<-createS.Exited

	Expect(createS).To(gexec.Exit(0))
}

func (runner *Runner) DropTestDB() {
	dropdb := exec.Command("dropdb", "-U", "postgres", "-p", strconv.Itoa(runner.Port), "testdb")
	dropS, err := gexec.Start(dropdb, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-dropS.Exited

	Expect(dropS).To(gexec.Exit(0))
}

func (runner *Runner) Truncate() {
	truncate := exec.Command(
		"psql",
		"-U", "postgres",
		"-p", strconv.Itoa(runner.Port),
		"testdb",
		"-c", `
			SET client_min_messages TO WARNING;

			CREATE OR REPLACE FUNCTION truncate_tables() RETURNS void AS $$
			DECLARE
					statements CURSOR FOR
							SELECT tablename FROM pg_tables
							WHERE schemaname = 'public' AND tablename != 'migrations_history';
			BEGIN
					FOR stmt IN statements LOOP
							EXECUTE 'TRUNCATE TABLE ' || quote_ident(stmt.tablename) || ' RESTART IDENTITY CASCADE;';
					END LOOP;
			END;
			$$ LANGUAGE plpgsql;

			CREATE OR REPLACE FUNCTION drop_ephemeral_sequences() RETURNS void AS $$
			DECLARE
					statements CURSOR FOR
							SELECT relname FROM pg_class
							WHERE relname LIKE 'build_event_id_seq_%';
			BEGIN
					FOR stmt IN statements LOOP
							EXECUTE 'DROP SEQUENCE ' || quote_ident(stmt.relname) || ';';
					END LOOP;
			END;
			$$ LANGUAGE plpgsql;

			CREATE OR REPLACE FUNCTION drop_ephemeral_tables() RETURNS void AS $$
			DECLARE
					statements CURSOR FOR
							SELECT relname FROM pg_class
							WHERE relname LIKE 'pipeline_build_events_%'
							AND relkind = 'r';
					team_statements CURSOR FOR
							SELECT relname FROM pg_class
							WHERE relname LIKE 'team_build_events_%'
							AND relkind = 'r';
			BEGIN
					FOR stmt IN statements LOOP
							EXECUTE 'DROP TABLE ' || quote_ident(stmt.relname) || ';';
					END LOOP;
					FOR stmt IN team_statements LOOP
							EXECUTE 'DROP TABLE ' || quote_ident(stmt.relname) || ';';
					END LOOP;
			END;
			$$ LANGUAGE plpgsql;

			CREATE OR REPLACE FUNCTION reset_global_sequences() RETURNS void AS $$
			DECLARE
					statements CURSOR FOR
							SELECT relname FROM pg_class
							WHERE relname IN ('one_off_name', 'config_version_seq');
			BEGIN
					FOR stmt IN statements LOOP
							EXECUTE 'ALTER SEQUENCE ' || quote_ident(stmt.relname) || ' RESTART WITH 1;';
					END LOOP;
			END;
			$$ LANGUAGE plpgsql;

			SELECT truncate_tables();
			SELECT drop_ephemeral_sequences();
			SELECT drop_ephemeral_tables();
			SELECT reset_global_sequences();
		`,
	)

	truncateS, err := gexec.Start(truncate, ginkgo.GinkgoWriter, ginkgo.GinkgoWriter)
	Expect(err).NotTo(HaveOccurred())

	<-truncateS.Exited

	Expect(truncateS).To(gexec.Exit(0))
}
