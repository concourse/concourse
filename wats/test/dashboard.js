import test from 'ava';
import Fly from './helpers/fly'
import Web from './helpers/web'
import puppeteer from 'puppeteer';

const Suite = require('./helpers/suite');

const color = require('color');
const palette = require('./helpers/palette');

test.beforeEach(async t => {
  t.context = await Suite.build(t);
});

test.afterEach(async t => {
  t.context.passed(t);
});

test.afterEach.always(async t => {
  await t.context.finish(t);
});

async function showsPipelineState(t, setup, assertions) {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');

  await setup(t);

  await t.context.web.page.goto(t.context.web.route('/dashboard'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.web.page.waitFor(`${group} .dashboard-pipeline`);
  const pipeline = await t.context.web.page.$(`${group} .dashboard-pipeline`);
  const text = await t.context.web.text(pipeline);

  const banner = await t.context.web.page.$(`${group} .dashboard-pipeline-banner`);
  const background = await t.context.web.computedStyle(banner, 'backgroundColor');

  await assertions(t, text, color(background), group);
};

test('does not show team name when unauthenticated and team has no exposed pipelines', async t => {
  t.context.web = await Web.build(t.context.url)
  await t.context.web.page.goto(t.context.web.route('/'));

  const group = `.dashboard-team-group[data-team-name="main"]`;
  const element = await t.context.web.page.$(group);

  t.falsy(element);
})

test('shows team name with no tag when unauthenticated and team has an exposed pipeline', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('expose-pipeline -p some-pipeline');

  t.context.web = await Web.build(t.context.url)
  await t.context.web.page.goto(t.context.web.route('/'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  const element = await t.context.web.page.waitFor(group);
  t.truthy(element);

  const tag = await t.context.web.page.$(`${group} .dashboard-team-tag`);
  t.falsy(tag);
});

test('shows team name with member tag when user is a member of the team', async t => {
  let teamName = await t.context.newTeam('test2');
  let userFly = await Fly.build(t.context.url, 'test2', 'test2', teamName);
  await userFly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');

  t.context.web = await Web.build(t.context.url, 'test2', 'test2');
  await t.context.web.login(t);

  const group = `.dashboard-team-group[data-team-name="${teamName}"] .dashboard-team-tag`;
  const element = await t.context.web.page.waitFor(group);
  t.truthy(element);

  const tagText = await t.context.web.page.$$eval(group, n => n[0].innerText);
  t.deepEqual(tagText, "MEMBER");
})

test('does not show team name when user is logged in another team and has no exposed pipelines', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');

  let teamName = await t.context.newTeam('test2');
  let myFly = await Fly.build(t.context.url, 'test2', 'test2', teamName);
  myFly.run('set-pipeline -n -p another-pipeline -c fixtures/states-pipeline.yml');

  t.context.web = await Web.build(t.context.url, 'test2', 'test2');
  await t.context.web.login(t);

  await t.context.web.page.waitFor('.dashboard-content');
  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  const element = await t.context.web.page.$(group);
  t.falsy(element);
})

test('shows team name with public tag when user is member of main team', async t => {
  let teamName = await t.context.newTeam('test2');
  let userFly = await Fly.build(t.context.url, 'test2', 'test2', teamName);
  await userFly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');

  await t.context.web.page.goto(t.context.web.route('/'));
  const group = `.dashboard-team-group[data-team-name="${teamName}"] .dashboard-team-tag`;
  const element = await t.context.web.page.waitFor(group);
  t.truthy(element);

  const tagText = await t.context.web.page.$$eval(group, n => n[0].innerText);
  t.deepEqual(tagText, "PUBLIC");
});

test('shows team name with public tag when team has an exposed pipeline and user is logged into another team', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('expose-pipeline -p some-pipeline');
  let teamName = await t.context.newTeam('test2');

  t.context.web = await Web.build(t.context.url, 'test2', 'test2');
  await t.context.web.login(t);

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"] .dashboard-team-tag`;
  const element = await t.context.web.page.waitFor(group);
  t.truthy(element);

  const tagText = await t.context.web.page.$$eval(group, n => n[0].innerText);
  t.deepEqual(tagText, "PUBLIC");
});

test('shows pipelines in their correct order', async t => {
  let pipelineOrder = ['first', 'second', 'third', 'fourth', 'fifth'];

  for (var i = 0; i < pipelineOrder.length; i++) {
    let name = pipelineOrder[i];
    await t.context.fly.run(`set-pipeline -n -p ${name} -c fixtures/states-pipeline.yml`);
  }

  await t.context.web.page.goto(t.context.web.route('/'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.web.page.waitFor(`${group} .pipeline-wrapper:nth-child(${pipelineOrder.length}) .dashboard-pipeline`);

  const names = await t.context.web.page.$$eval(`${group} .dashboard-pipeline-name`, nameElements => {
    var names = [];
    nameElements.forEach(e => names.push(e.innerText));
    return names;
  });

  t.deepEqual(names, pipelineOrder);
});

test('shows pipelines with no finished builds in grey', showsPipelineState, async t => {
  // no setup
}, (t, text, background) => {
  t.regex(text, /some-pipeline/);
  t.regex(text, /pending/);

  t.deepEqual(background, palette.grey);
});

test('shows paused pipelines in blue', showsPipelineState, async t => {
  await t.context.fly.run("pause-pipeline -p some-pipeline");
}, (t, text, background) => {
  t.regex(text, /some-pipeline/);
  t.regex(text, /paused/);

  t.deepEqual(background, palette.blue);
});

test('shows pipelines with only passing builds in green', showsPipelineState, async t => {
  await t.context.fly.run("trigger-job -w -j some-pipeline/passing");
}, (t, text, background) => {
  t.regex(text, /some-pipeline/);
  t.deepEqual(background, palette.green);
});

test('shows pipelines with any failed builds in red', showsPipelineState, async t => {
  await t.context.fly.run("trigger-job -w -j some-pipeline/passing");
  await t.throws(t.context.fly.run("trigger-job -w -j some-pipeline/failing"));
}, (t, text, background) => {
  t.regex(text, /some-pipeline/);
  t.deepEqual(background, palette.red);
});

test('shows pipelines with any errored builds in amber', showsPipelineState, async t => {
  await t.context.fly.run("trigger-job -w -j some-pipeline/passing");
  await t.throws(t.context.fly.run("trigger-job -w -j some-pipeline/erroring"));
}, (t, text, background) => {
  t.regex(text, /some-pipeline/);
  t.deepEqual(background, palette.amber);
});

test('shows pipelines with any aborted builds in brown', showsPipelineState, async t => {
  await t.context.fly.run("trigger-job -j some-pipeline/passing -w");

  let run = t.context.fly.spawn("trigger-job -j some-pipeline/running -w");

  run.childProcess.stdout.on('data', async data => {
    if (data.toString().indexOf("hello") !== -1) {
      await t.context.fly.run("abort-build -j some-pipeline/running -b 1");
    }
  });

  await t.throws(run);
}, (t, text, background) => {
  t.deepEqual(background, palette.brown);
});

test('auto-refreshes to reflect state changes', showsPipelineState, async t => {
  await t.context.fly.run("trigger-job -w -j some-pipeline/passing");
}, async (t, text, background, group) => {
  t.deepEqual(background, palette.green);

  await t.throws(t.context.fly.run("trigger-job -w -j some-pipeline/failing"));

  await t.context.web.page.waitFor(10000);

  let newBanner = await t.context.web.page.$(`${group} .dashboard-pipeline-banner`);
  let newBackground = await t.context.web.computedStyle(newBanner, 'backgroundColor');
  t.deepEqual(color(newBackground), palette.red);
});

test('links to specific builds', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');
  await t.context.fly.run("trigger-job -w -j some-pipeline/passing");

  await t.context.web.page.goto(t.context.web.route('/dashboard'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.web.clickAndWait(`${group} .node[data-tooltip="passing"] a`, '.build-header');
  t.regex(await t.context.web.text(), /passing #1/);
});
