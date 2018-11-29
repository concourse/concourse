'use strict';

const { exec, spawn } = require('child-process-promise');
const uuidv4 = require('uuid/v4');
const tmp = require('tmp-promise');

class Fly {
  constructor(url, username, password, teamName) {
    this.url = url;
    this.username = username;
    this.password = password;
    this.teamName = teamName;
    this.target = `wats-target-${uuidv4()}`;
  }

  static async build(url, username, password, teamName) {
    let fly = new Fly(url, username, password, teamName);
    await fly.init();
    return fly;
  }

  async setupHome() {
    this.home = await tmp.dir({ unsafeCleanup: true });
  }

  async init() {
    await this.setupHome();
    await this.loginAs(this.teamName);
  }

  destroyTeam(teamName) {
    return this.run(`destroy-team --team-name ${teamName} --non-interactive`)
  }

  run(command) {
    return this._run(`fly -t ${this.target} ${command}`);
  }

  spawn(command) {
    return this._spawn('fly', ['-t', this.target].concat(command.split(' ')));
  }

  newTeam(teamName, username) {
    return this.run(`set-team --team-name ${teamName} --local-user=${username} --non-interactive`);
  }

  async cleanup() {
    await this.home.cleanup();
  }

  loginAs(teamName) {
    return this.run(`login -c ${this.url} -n ${teamName} -u ${this.username} -p ${this.password}`);
  }

  _run(command) {
    return exec(command, {env: this._env()});
  }

  _spawn(path, args) {
    return spawn(path, args, {env: this._env()});
  }

  _env() {
    return {
      "HOME": this.home.path,
      "PATH": process.env["PATH"]
    }
  }
}

module.exports = Fly;
