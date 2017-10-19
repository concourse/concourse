Feature('Dashboard');

var teamName;
var otherTeamName;

BeforeSuite((I) => {
  I.cleanUpTestTeams();
});

Before(function*(I) {
  I.flyLoginAs("main");

  teamName = yield I.grabANewTeam();
  otherTeamName = yield I.grabANewTeam();

  I.flyLoginAs(teamName);
  I.fly("set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml");
  I.fly("unpause-pipeline -p some-pipeline");

  I.flyLoginAs(otherTeamName);
  I.fly("set-pipeline -n -p other-pipeline-private -c fixtures/states-pipeline.yml");
  I.fly("unpause-pipeline -p other-pipeline-private");

  I.fly("set-pipeline -n -p other-pipeline-public -c fixtures/states-pipeline.yml");
  I.fly("unpause-pipeline -p other-pipeline-public");
  I.fly("expose-pipeline -p other-pipeline-public");

  I.flyLoginAs(teamName);
});

Scenario("shows current team first, followed by other team's public pipelines", (I) => {
  I.loginAs(teamName);

  I.amOnPage("/dashboard");
  I.waitForElement('.dashboard-team-group');

  within('.dashboard-team-group:nth-child(1)', () => {
    I.see(teamName);
    I.see('some-pipeline');
  });

  within(`.dashboard-team-group[data-team-name="${otherTeamName}"]`, () => {
    I.see(otherTeamName);
    I.see('other-pipeline-public');
    I.dontSee('other-pipeline-private');
  });
});
