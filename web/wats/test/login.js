import test from 'ava';
import Fly from './helpers/fly';
import Web from './helpers/web';
import puppeteer from 'puppeteer';
import Suite from './helpers/suite';

test.beforeEach(async t => {
  let url = process.env.ATC_URL || 'http://localhost:8080';
  let username = process.env.ATC_ADMIN_USERNAME || 'test';
  let password = process.env.ATC_ADMIN_PASSWORD || 'test';
  let teamName = 'main';
  let context = {};
  context.url = url;
  context.fly = new Fly(url, username, password, teamName);
  await context.fly.setupHome();
  context.web = await Web.build(url, username, password);
  context.succeeded = false;
  t.context = context;
});

test.afterEach(async t => {
  t.context.succeeded = true;
});

test.afterEach.always(async t => {
  await t.context.fly.run('destroy-pipeline -n -p some-pipeline');
  if (t.context.web.page && !t.context.succeeded) {
    await t.context.web.page.screenshot({path: 'failure.png'});
  }

  if (t.context.web.browser) {
    await t.context.web.browser.close();
  }
  await t.context.fly.cleanup();
});

test('can fly login with browser and reuse same browser without CSRF issues', async t => {
  let flyPromise = t.context.fly.spawn(`login -c ${t.context.url}`);
  flyPromise.childProcess.stdout.on('data', async data => {
    data.toString().split("\n").forEach(async line => {
      if (line.includes(t.context.url)) {
        await t.context.web.page.goto(line);
        await t.context.web.performLogin();
        await t.context.web.page.click('#submit-login');
      }
    });
  });
  await flyPromise;
  let currentUrl = t.context.web.page.url();
  t.true(currentUrl.includes(`${t.context.url}/fly_success`));
  await t.context.fly.run('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml');
  await t.context.web.page.goto(t.context.web.route('/'));
  let pipelineSelector = '.card[data-pipeline-name=some-pipeline]';
  let playButton = `${pipelineSelector} [style*="ic_play"]`;
  let pauseButton = `${pipelineSelector} [style*="ic_pause"]`;
  await t.context.web.page.waitFor(playButton);
  await t.context.web.page.click(playButton);
  await t.context.web.page.waitForSelector(pauseButton, {timeout: 90000});
  t.pass();
});
