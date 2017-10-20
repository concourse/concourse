let expect = require('chai').expect;

Feature('Dashboard');

var teamName;
let pipelineOrder = ['first', 'second', 'third', 'fourth', 'fifth'];

BeforeSuite((I) => {
  I.cleanUpTestTeams();
});

Before(function*(I) {
  teamName = yield I.grabANewTeam();

  I.flyLoginAs(teamName);

  pipelineOrder.forEach((name) => {
    I.fly(`set-pipeline -n -p ${name} -c fixtures/states-pipeline.yml`);
  });

  I.loginAs(teamName);
  I.amOnPage("/dashboard");
});

Scenario("shows pipelines in their correct order", function*(I) {
  var teamGroup = `.dashboard-team-group[data-team-name="${teamName}"]`;
  I.waitForElement(teamGroup);

  within(teamGroup, function* () {
    I.waitForElement(`.dashboard-pipeline:nth-child(${pipelineOrder.length})`);

    let names = yield I.executeScript(() => {
      var names = [];

      document.querySelectorAll('.dashboard-pipeline-name').forEach((e) => {
        names.push(e.innerText);
      });

      return names;
    });

    expect(names).to.eql(pipelineOrder);
  });
});
