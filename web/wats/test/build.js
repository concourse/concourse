import test from 'ava';

const Suite = require('./helpers/suite');

const color = require('color');
const palette = require('./helpers/palette');

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

test('shows abort hooks', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/hooks-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');

  await t.context.fly.run('trigger-job -j some-pipeline/on_abort');

  await t.context.web.page.goto(t.context.web.route(`/teams/${t.context.teamName}/pipelines/some-pipeline/jobs/on_abort/builds/1`));
  await t.context.web.page.setViewport({width: 1200, height: 900});

  await t.context.web.waitForText("say-bye-from-step");
  await t.context.web.waitForText("say-bye-from-job");
  await t.context.web.waitForText("looping");

  await t.context.web.clickAndWait('button[title="Abort Build"]', '.build-header[style*="rgb(139, 87, 42)"]'); // brown
  await t.context.web.page.waitFor('[data-step-name="say-bye-from-step"] [data-step-state="succeeded"]');
  await t.context.web.page.waitFor('[data-step-name="say-bye-from-job"] [data-step-state="succeeded"]');

  await t.context.web.clickAndWait('[data-step-name="say-bye-from-step"] .header', '[data-step-name="say-bye-from-step"] .step-body:not(.step-collapsed)');
  t.regex(await t.context.web.text(), /bye from step/);

  await t.context.web.clickAndWait('[data-step-name="say-bye-from-job"] .header', '[data-step-name="say-bye-from-job"] .step-body:not(.step-collapsed)');
  t.regex(await t.context.web.text(), /bye from job/);
});

test('can be switched between', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');

  await t.context.fly.run('trigger-job -w -j some-pipeline/passing');
  await t.context.fly.run('trigger-job -w -j some-pipeline/passing');

  await t.context.web.page.goto(t.context.web.route(`/teams/${t.context.teamName}/pipelines/some-pipeline/jobs/passing/builds/1`));

  await t.context.web.clickAndWait('#builds li:nth-child(1) a', '[data-build-name="2"]');
  t.regex(await t.context.web.text(), /passing #2/);

  await t.context.web.clickAndWait('#builds li:nth-child(2) a', '[data-build-name="1"]');
  t.regex(await t.context.web.text(), /passing #1/);
});

test('shows error log at the bottom of an erroring build', async t => {
  await waitForErroringBuild(t);
  let errorStep = await t.context.web.page.waitFor('[data-step-name="error"]');
  t.regex(await t.context.web.text(errorStep), /banana/);
});

test('hovering erroring step header shows non-negative duration', async t => {
  await waitForErroringBuild(t);

  let triangleSelector = '[style*="exclamation-triangle"]';
  let triangle = await t.context.web.page.hover(triangleSelector);
  await t.context.web.page.waitFor(`${triangleSelector} table`);
  t.notRegex(await t.context.web.text(triangle), /-\d+/);
});

async function waitForErroringBuild(t) {
  let fixture = 'fixtures/states-pipeline.yml'
  await t.context.fly.run(`set-pipeline -n -p some-pipeline -c ${fixture}`);
  await t.context.fly.run('unpause-pipeline -p some-pipeline');

  await t.context.fly.run('trigger-job -j some-pipeline/erroring');

  let pipelinePath = `/teams/${t.context.teamName}/pipelines/some-pipeline`;
  let buildPath = `jobs/erroring/builds/1`;
  let path = `${pipelinePath}/${buildPath}`
  await t.context.web.page.goto(t.context.web.route(path));

  let amberHeader = '#build-header[style*="rgb(245, 166, 35)"]';
  await t.context.web.page.waitFor(amberHeader);
}
