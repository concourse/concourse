const Web = require('../../wats/helpers/web');

class Benchmark {
  constructor() {
    this.url = `file://${process.argv[2] || '/tmp/benchmark.html'}`;
    this.web = new Web(this.url);
  }

  async run() {
    await this.web.init();
    await this.web.page.goto(this.url);
    await this.web.waitForText('Benchmark Report');
    const bodyHandle = await this.web.page.$('body > div > div');
    const html = await this.web.page.evaluate(body => body.innerText, bodyHandle);
    await bodyHandle.dispose();
    console.log(html);
  }
}

async function main() {
  await new Benchmark().run();
  process.exit(0);
}

main();
