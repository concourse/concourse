import test from 'ava';
import Fly from '../helpers/fly'
import Web from '../helpers/web'
import puppeteer from 'puppeteer';

const Suite = require('../helpers/suite');

const color = require('color');
const palette = require('../helpers/palette');

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

async function showsPipelineState(t, setup, assertions) {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('unpause-pipeline -p some-pipeline');

  await setup(t);

  await t.context.web.page.goto(t.context.web.route('/'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.web.page.waitFor(`${group} .card`);
  const pipeline = await t.context.web.page.$(`${group} .card`);
  const text = await t.context.web.text(pipeline);

  const banner = await t.context.web.page.$(`${group} .banner`);
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

test('does not show team name when user is logged in another non-main team and has no exposed pipelines', async t => {
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.fly.run('login -n ' + t.context.guestTeamName + ' -u '+ t.context.guestUsername +' -p ' + t.context.guestPassword);
  await t.context.fly.run('set-pipeline -n -p non-main-pipeline -c fixtures/states-pipeline.yml');

  let web = await Web.build(t.context.url, t.context.guestUsername, t.context.guestPassword);
  await web.login(t);
  await web.page.goto(web.route('/'));
  const myGroup = `.dashboard-team-group[data-team-name="${t.context.guestTeamName}"]`;
  const otherGroup = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await web.page.waitFor(myGroup);
  const element = await web.page.$(otherGroup);
  t.falsy(element);
})

test('shows pipelines in their correct order', async t => {
  let pipelineOrder = ['first', 'second', 'third', 'fourth', 'fifth'];

  for (var i = 0; i < pipelineOrder.length; i++) {
    let name = pipelineOrder[i];
    await t.context.fly.run(`set-pipeline -n -p ${name} -c fixtures/states-pipeline.yml`);
  }

  await t.context.web.page.goto(t.context.web.route('/'));

  const group = `.dashboard-team-group[data-team-name="${t.context.teamName}"]`;
  await t.context.web.page.waitFor(`${group} .pipeline-wrapper:nth-child(${pipelineOrder.length}) .card`);

  const names = await t.context.web.page.$$eval(`${group} .dashboard-pipeline-name`, nameElements => {
    var names = [];
    nameElements.forEach(e => names.push(e.innerText));
    return names;
  });

  t.deepEqual(names, pipelineOrder);
});

test('auto-refreshes to reflect state changes', showsPipelineState, async t => {
  await t.context.fly.run("trigger-job -w -j some-pipeline/passing");
}, async (t, text, background, group) => {
  t.deepEqual(background, palette.green);

  await t.throwsAsync(async () => await t.context.fly.run("trigger-job -w -j some-pipeline/failing"));

  await t.context.web.page.waitFor(10000);

  let newBanner = await t.context.web.page.$(`${group} .banner`);
  let newBackground = await t.context.web.computedStyle(newBanner, 'backgroundColor');
  t.deepEqual(color(newBackground), palette.red);
});

test('picks up cluster name from configuration', async t => {
  await t.context.web.page.goto(t.context.web.route('/'));
  const clusterName = await t.context.web.page.$eval(`#top-bar-app > div:nth-child(1)`, el => el.innerText);

  t.is(clusterName, 'dev');
});
