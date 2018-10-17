import test from 'ava';

const Suite = require('./helpers/suite');

test.beforeEach(async t => {
  t.context = await Suite.build(t);
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

  await t.context.fly.run('trigger-job -j some-pipeline/say-hello -w');

  await t.context.web.page.goto(t.context.web.route(`/`));
  await t.context.web.waitForText('some-pipeline');

  await t.context.web.page.goto(t.context.web.route('/teams/'+t.context.teamName+'/pipelines/some-pipeline`));
  await t.context.web.waitForText('say-hello');

  await t.context.web.page.goto(t.context.web.route('/teams/'+t.context.teamName+'/pipelines/some-pipeline/builds/1`));
  await t.context.web.page.waitFor('.build-header.succeeded');
  await t.context.web.clickAndWait('[data-step-name="hello"] .header', '[data-step-name="hello"] .step-body:not(.step-collapsed)');
  await t.context.web.waitForText('Hello, world!');

  t.true(true);
});

test('running one-off builds', async t => {
  await t.context.fly.run('execute -c fixtures/smoke-task.yml');
  t.true(true);
});
