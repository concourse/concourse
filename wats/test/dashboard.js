import test from 'ava';

const Suite = require('./helpers/suite');

const color = require('color');
const palette = require('./helpers/palette');

test.beforeEach(async t => {
  t.context = new Suite();
  await t.context.start(t);
});

test.afterEach(async t => {
  t.context.passed(t);
});

test.always.afterEach(async t => {
  await t.context.finish(t);
});

async function showsPipelineState(t, setup, assertions) {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');

  await setup(t);

  await t.context.page.goto(t.context.web.route('/dashboard'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.page.waitFor(`${group} .dashboard-pipeline`);
  const pipeline = await t.context.page.$(`${group} .dashboard-pipeline`);
  const text = await t.context.web.text(t.context.page, pipeline);

  const banner = await t.context.page.$(`${group} .dashboard-pipeline-banner`);
  const background = await t.context.web.computedStyle(t.context.page, banner, 'backgroundColor');

  await assertions(t, text, color(background), group);
};

test('shows pipelines in their correct order', async t => {
  let pipelineOrder = ['first', 'second', 'third', 'fourth', 'fifth'];

  for (var i = 0; i < pipelineOrder.length; i++) {
    let name = pipelineOrder[i];
    await t.context.fly.run(`set-pipeline -n -p ${name} -c fixtures/states-pipeline.yml`);
  }

  await t.context.page.goto(t.context.web.route('/dashboard'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.page.waitFor(`${group} .dashboard-pipeline:nth-child(${pipelineOrder.length})`);

  const names = await t.context.page.$$eval(`${group} .dashboard-pipeline-name`, nameElements => {
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

  await t.context.page.waitFor(5000);

  let newBanner = await t.context.page.$(`${group} .dashboard-pipeline-banner`);
  let newBackground = await t.context.web.computedStyle(t.context.page, newBanner, 'backgroundColor');
  t.deepEqual(color(newBackground), palette.red);
});

test('links to specific builds', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');
  await t.context.fly.run("trigger-job -w -j some-pipeline/passing");

  await t.context.page.goto(t.context.web.route('/dashboard'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.web.clickAndWait(t.context.page, `${group} .node[data-tooltip="passing"] a`, '.build-header');
  t.regex(await t.context.web.text(t.context.page), /passing #1/);
});
