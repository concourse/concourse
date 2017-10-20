let color = require('color');
let expect = require('chai').expect;

let palette = require('./../../palette.js');

Feature('Build keyboard shortcuts');

var teamName;

BeforeSuite((I) => {
  I.cleanUpTestTeams();
});

Before(function*(I) {
  I.flyLoginAs("main");

  teamName = yield I.grabANewTeam();

  I.flyLoginAs(teamName);
  I.loginAs(teamName);

  I.fly("set-pipeline -n -p some-pipeline -c fixtures/pipeline-with-long-output.yml");
  I.fly("unpause-pipeline -p some-pipeline");
});

Scenario('scrolls to the top with gg', function*(I) {
  I.fly("trigger-job -j some-pipeline/long-output");

  I.amOnPage(`/teams/${teamName}/pipelines/some-pipeline/jobs/long-output/builds/1`);
  I.resizeWindow(1024, 768);
  I.waitForText('Line 100', 30);

  I.pressKey('G');

  let pos1 = yield I.grabScrollPosition();
  expect(pos1).to.be.gt(0);

  I.pressKey('g');
  I.pressKey('g');

  let pos2 = yield I.grabScrollPosition();
  expect(pos2).to.equal(0);
});
