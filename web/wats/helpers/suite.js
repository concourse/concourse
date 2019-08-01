const Fly = require('./fly');
const Web = require('./web');
const uuidv4 = require('uuid/v4');

// silence warning caused by starting many puppeteer
process.setMaxListeners(Infinity);

class Suite {
  constructor() {
    this.url = process.env.ATC_URL || 'http://localhost:8080';
    this.adminUsername = process.env.ATC_ADMIN_USERNAME || 'test';
    this.adminPassword = process.env.ATC_ADMIN_PASSWORD || 'test';
    this.guestUsername = process.env.ATC_GUEST_USERNAME || 'guest';
    this.guestPassword = process.env.ATC_GUEST_PASSWORD || 'guest';

    this.teamName = `watsjs-team-${uuidv4()}`;
    this.guestTeamName = `watsjs-non-main-team-${uuidv4()}`;
    this.teams = [];

    this.fly = new Fly(this.url, this.adminUsername, this.adminPassword, this.teamName);
    this.web = new Web(this.url, this.adminUsername, this.adminPassword);
  }

  async init(t) {
    await this.newTeam(this.adminUsername, this.teamName);
    await this.newTeam(this.guestUsername, this.guestTeamName);
    await this.fly.init();
    await this.web.init();
    await this.web.login(t);

    this.succeeded = false;
  }

  async newTeam(username = this.adminUsername, teamName) {
    if (!teamName) {
      teamName = `watsjs-team-${uuidv4()}`;
    }
    let fly = await Fly.build(this.url, this.adminUsername, this.adminPassword, 'main');

    await fly.newTeam(teamName, username);
    this.teams.push(teamName);

    return teamName;
  }

  async destroyTeams() {
    let fly = await Fly.build(this.url, this.adminUsername, this.adminPassword, 'main');

    var team;
    while (team = this.teams.pop()) {
      await fly.destroyTeam(team);
    }
  }

  passed(t) {
    this.succeeded = true;
  }

  async finish(t) {
    if (this.web.page && !this.succeeded) {
      await this.web.page.screenshot({path: 'failure.png'});
    }

    if (this.web.browser) {
      await this.web.browser.close();
    }

    await this.destroyTeams();
    await this.fly.cleanup();
  }
}

module.exports = Suite;
