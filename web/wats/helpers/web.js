'use strict';

const puppeteer = require('puppeteer');
const { exec } = require('child-process-promise');
const uuidv4 = require('uuid/v4');

class Web {
  constructor(url, username, password) {
    this.url = url;
    this.username = username;
    this.password = password;
  }

  static async build(url, username, password) {
    let web = new Web(url, username, password);
    await web.init();
    return web;
  }

  async init() {
    this.browser = await puppeteer.launch({
      //headless: false,
      args: ['--no-sandbox', '--disable-setuid-sandbox']
    });

    this.page = await this.browser.newPage();
    //Default page navigation timeout to 90 Seconds.
    this.page.setDefaultNavigationTimeout(90000);
    this.page.on("console", msg => {
      console.log(`BROWSER (${msg.type()}):`, msg.text());
    });
  }

  route(path) {
    return `${this.url}${path}`;
  }

  text(ele) {
    return this.page.evaluate(x => (x || document.body).innerText, ele);
  }

  waitForText(text) {
    return this.page.waitForFunction((text) => {
      return document.body.innerText.indexOf(text) !== -1
    }, {
      polling: 100,
      timeout: 90000
    }, text)
  }

  async waitForBackgroundColor(selector, backgroundColor, {timeout = 30000} = {}) {
    await this.page.waitFor(({expectedBackground, selector}) => {
      const elem = document.querySelector(selector);
      if (elem === null) return false;
      const background = elem.style.backgroundColor;
      return background === expectedBackground;
    }, {timeout}, {
      selector,
      expectedBackground: backgroundColor.rgb().string(),
    });
  }

  async scrollIntoView(selector) {
    await this.page.waitFor(selector);
    await this.page.evaluate(selector => {
      const elem = document.querySelector(selector);
      elem.scrollIntoView(true);
    }, selector);
  }

  async login(t) {
    await this.visitLoginPage();
    await this.performLogin();
    await this.clickAndWait('#submit-login', '#user-id');
    t.notRegex(await this.text(), /login/);
  }

  async visitLoginPage() {
    await this.page.goto(`${this.url}/sky/login`);
  }

  async performLogin() {
    await this.page.waitForSelector('#login', {timeout: 90000});
    await this.page.type('#login', this.username);
    await this.page.type('#password', this.password);
  }

  async clickAndWait(clickSelector, waitSelector) {
    await this.page.waitFor(clickSelector);
    await this.page.click(clickSelector);
    await this.page.waitForSelector(waitSelector, {timeout: 90000});
  }

  computedStyle(element, style) {
    return element.executionContext().evaluate((element, style) => {
      return window.getComputedStyle(element)[style]
    }, element, style)
  }
}

module.exports = Web;
