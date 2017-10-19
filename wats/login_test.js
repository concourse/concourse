Feature('Login');

Scenario('logging in', (I) => {
  I.amOnPage('/teams/main/login');
  I.waitForText('login', 1, '.login-page');
  I.click('login', '.login-page');
  I.dontSee('login');
});
