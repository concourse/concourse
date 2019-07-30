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
  await setupPipeline(t);
  await pinVersion(t);
  await resetVersionsList(t);
  await unpinVersionUsingTopBar(t);
  await reloadPageAndCheckResourceIsStillNotPinned(t);
  t.pass();
});

async function setupPipeline(t) {
  await t.context.fly.run('set-pipeline -n -p pipeline -c fixtures/before-pin.yml');
  await t.context.fly.run('unpause-pipeline -p pipeline');
  await t.context.fly.run('check-resource -r pipeline/resource');
}

async function pinVersion(t) {
  await goToResourcePage(t);
  await t.context.web.clickAndWait(pinButtonSelector, versionInPinBarSelector);
}

async function resetVersionsList(t) {
  await t.context.fly.run('set-pipeline -n -p pipeline -c fixtures/after-pin.yml');
  await t.context.fly.run('check-resource -r pipeline/resource');
}

async function unpinVersionUsingTopBar(t) {
  await goToResourcePage(t);
  await t.context.web.page.waitFor(topBarPinIconSelector);
  await t.context.web.page.click(topBarPinIconSelector);
  await checkNoVersionInPinBar(t);
}

async function reloadPageAndCheckResourceIsStillNotPinned(t) {
  await goToResourcePage(t);
  await checkNoVersionInPinBar(t);
}

async function goToResourcePage(t) {
  let url = `/teams/${t.context.teamName}/pipelines/pipeline/resources/resource`;
  await t.context.web.page.goto(t.context.web.route(url));
  await waitForPageLoad(t);
}

async function checkNoVersionInPinBar(t) {
  await t.context.web.page.waitFor(() => !document.querySelector('#pin-bar table'));
}

async function waitForPageLoad(t) {
  await t.context.web.page.waitFor(pinButtonSelector);
}
