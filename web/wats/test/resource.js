import test from 'ava';
import Suite from '../helpers/suite';

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

const pinButtonSelector = '[aria-label="Pin Resource Version"]';
const versionInPinBarSelector = '#pin-bar table';
const topBarPinIconSelector = '#pin-icon';

test('can unpin from top bar when pinned version is not in the versions list', async t => {
  const pipelineId = await setupPipeline(t);
  await pinVersion(t, pipelineId);
  await resetVersionsList(t);
  await unpinVersionUsingTopBar(t, pipelineId);
  await reloadPageAndCheckResourceIsStillNotPinned(t, pipelineId);
  t.pass();
});

async function setupPipeline(t) {
  const pipelineId = await t.context.fly.setPipeline('pipeline', 'fixtures/before-pin.yml');
  await t.context.fly.run('unpause-pipeline -p pipeline');
  await t.context.fly.run('check-resource -r pipeline/resource');
  return pipelineId;
}

async function pinVersion(t, pipelineId) {
  await goToResourcePage(t, pipelineId);
  await t.context.web.clickAndWait(pinButtonSelector, versionInPinBarSelector);
}

async function resetVersionsList(t) {
  const pipelineId = await t.context.fly.setPipeline('pipeline', 'fixtures/after-pin.yml');
  await t.context.fly.run('check-resource -r pipeline/resource');
  return pipelineId;
}

async function unpinVersionUsingTopBar(t, pipelineId) {
  await goToResourcePage(t, pipelineId);
  await t.context.web.page.waitFor(topBarPinIconSelector);
  await t.context.web.page.click(topBarPinIconSelector);
  await checkNoVersionInPinBar(t);
}

async function reloadPageAndCheckResourceIsStillNotPinned(t, pipelineId) {
  await goToResourcePage(t, pipelineId);
  await checkNoVersionInPinBar(t);
}

async function goToResourcePage(t, pipelineId) {
  const url = `/pipelines/${pipelineId}/resources/resource`;
  await t.context.web.page.goto(t.context.web.route(url));
  await waitForPageLoad(t);
}

async function checkNoVersionInPinBar(t) {
  await t.context.web.page.waitFor(() => !document.querySelector('#pin-bar table'));
}

async function waitForPageLoad(t) {
  await t.context.web.page.waitFor(pinButtonSelector);
}
