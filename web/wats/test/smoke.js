import test from 'ava';

const Suite = require('../helpers/suite');

test.beforeEach(async t => {
  t.context = new Suite();
  await t.context.init(t);
});

test.afterEach(async t => {
  t.context.passed(t);
});

test.afterEach.always(async t => {
  await t.context.finish(t);
});

test('running pipelines', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/smoke-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');

  var result = await t.context.fly.run('trigger-job -j some-pipeline/say-hello -w');
  t.regex(result.stdout, /Hello, world!/);
  t.regex(result.stdout, /pushing version: put-version/);

  await t.context.web.page.goto(t.context.web.route(`/`));
  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.web.scrollIntoView(group);
  await t.context.web.waitForText('some-pipeline');

  await t.context.web.page.goto(t.context.web.route(`/teams/${t.context.teamName}/pipelines/some-pipeline`));
  await t.context.web.waitForText('say-hello');

  await t.context.web.page.goto(t.context.web.route(`/teams/${t.context.teamName}/pipelines/some-pipeline/jobs/say-hello/builds/1`));
  await t.context.web.page.waitFor('.build-header[style*="rgb(17, 197, 96)"]'); // green
  await t.context.web.clickAndWait('[data-step-name="hello"] .header', '[data-step-name="hello"] .step-body:not(.step-collapsed)');
  await t.context.web.waitForText('Hello, world!');

  t.true(true);
});

test('running one-off builds', async t => {
  var result = await t.context.fly.run('execute -c fixtures/smoke-task.yml -i some-input=fixtures/some-input');
  t.regex(result.stdout, /Hello, world!/);
});

test('reaching the internet', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/smoke-internet-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');

  var result = await t.context.fly.run('trigger-job -j some-pipeline/use-the-internet -w');
  t.regex(result.stdout, /Hello, world!/);
});
