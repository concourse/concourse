'use strict';

const { exec } = require('child-process-promise');
const uuidv4 = require('uuid/v4');

class Web {
  constructor(url, username, password) {
    this.url = url;
    this.username = username;
    this.password = password;
  }

  route(path) {
    return `${this.url}${path}`;
  }

  betaRoute(path) {
    return `${this.url}/beta${path}`;
  }

  text(page, ele) {
    return page.evaluate(x => (x || document.body).innerText, ele);
  }

  waitForText(page, text) {
    return page.waitForFunction((text) => {
      return document.body.innerText.indexOf(text) !== -1
    }, {
      polling: 100,
      timeout: 60000
    }, text)
  }

  async login(t, page) {
    await page.goto(`${this.url}/sky/login`);
    await this.clickAndWait(page, '.dex-btn-icon--local')
    await page.type('#login', this.username);
    await page.type('#password', this.password);
    await this.clickAndWait(page, '#submit-login')
    t.notRegex(await this.text(page), /login/);
  }

  async clickAndWait(page, selector) {
    await page.waitFor(selector);
    await page.click(selector);
    await page.waitForNavigation({
      waitUntil: 'networkidle',
      networkIdleInflight: 0,
      timeout: 60000
    });
  }

  computedStyle(page, element, style) {
    return element.executionContext().evaluate((element, style) => {
      return window.getComputedStyle(element)[style]
    }, element, style)
  }
}

module.exports = Web;
