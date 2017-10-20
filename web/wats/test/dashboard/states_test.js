let assert = require('assert');
let color = require('color');
let expect = require('chai').expect;

let palette = require('./../../palette.js');

Feature('Dashboard states');

var teamName;

BeforeSuite((I) => {
  I.cleanUpTestTeams();
});

Before(function*(I) {
  I.flyLoginAs("main");

  teamName = yield I.grabANewTeam();

  I.flyLoginAs(teamName);
  I.fly("set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml");
  I.fly("unpause-pipeline -p some-pipeline");

  I.loginAs(teamName);
});

function* matchPipelineColor(I, expectedState, expectedColor) {
  let backgroundColor = yield I.grabComputedStyle('.dashboard-pipeline-banner', 'backgroundColor');
  expect(color(backgroundColor)).to.eql(expectedColor);
};

Scenario('shows pipelines with no finished builds in grey', (I) => {
  I.amOnPage('/dashboard');
  I.waitForElement('.dashboard-pipeline');

  within('.dashboard-pipeline', () => {
    I.see("some-pipeline");
    I.see("pending");

    matchPipelineColor(I, palette.base07);
  });
});

Scenario('shows paused pipelines in blue', (I) => {
  I.fly("pause-pipeline -p some-pipeline");

  I.amOnPage('/dashboard');
  I.waitForElement('.dashboard-pipeline');

  within('.dashboard-pipeline', () => {
    I.see("some-pipeline");
    I.see("paused");

    matchPipelineColor(I, palette.blue);
  });
});

Scenario('shows pipelines with only passing builds in green', (I) => {
  I.fly("trigger-job -w -j some-pipeline/passing");

  I.amOnPage('/dashboard');
  I.waitForElement('.dashboard-pipeline');

  within('.dashboard-pipeline', () => {
    I.see("some-pipeline");
    matchPipelineColor(I, palette.green);
  });
});

Scenario('shows pipelines with any failed builds in red', (I) => {
  I.fly("trigger-job -w -j some-pipeline/passing");
  I.flyExpectingFailure("trigger-job -w -j some-pipeline/failing");

  I.amOnPage('/dashboard');
  I.waitForElement('.dashboard-pipeline');

  within('.dashboard-pipeline', () => {
    I.see("some-pipeline");
    matchPipelineColor(I, palette.red);
  });
});

Scenario('shows pipelines with any errored builds in orange', (I) => {
  I.fly("trigger-job -w -j some-pipeline/passing");
  I.flyExpectingFailure("trigger-job -w -j some-pipeline/erroring");

  I.amOnPage('/dashboard');
  I.waitForElement('.dashboard-pipeline');

  within('.dashboard-pipeline', () => {
    I.see("some-pipeline");
    matchPipelineColor(I, palette.orange);
  });
});

Scenario('shows pipelines with any aborted builds in brown', (I) => {
  I.fly("trigger-job -j some-pipeline/passing -w");
  I.fly("trigger-job -j some-pipeline/running");
  I.fly("abort-build -j some-pipeline/running -b 1");

  I.amOnPage('/dashboard');
  I.waitForElement('.dashboard-pipeline');

  within('.dashboard-pipeline', () => {
    I.see("some-pipeline");
    matchPipelineColor(I, palette.brown);
  });
});

Scenario('auto-refreshes to reflect state changes', (I) => {
  I.fly("trigger-job -w -j some-pipeline/passing");

  I.amOnPage('/dashboard');
  I.waitForElement('.dashboard-pipeline');

  within('.dashboard-pipeline', () => {
    I.see("some-pipeline");
    matchPipelineColor(I, palette.green);
  });

  I.flyExpectingFailure("trigger-job -w -j some-pipeline/failing");

  I.wait(5);

  within('.dashboard-pipeline', () => {
    I.see("some-pipeline");
    matchPipelineColor(I, palette.red);
  });
});
