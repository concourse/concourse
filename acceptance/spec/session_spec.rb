describe 'session', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home)  { Dir.mktmpdir }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')
  end

  context 'when logging in' do
    it 'hides the tokens after redirect' do
      visit dash_route
      expect(page).to have_content 'login'

      click_on 'login'
      expect(page).to have_content 'username'

      fill_in 'login', with: ATC_USERNAME
      fill_in 'password', with: ATC_PASSWORD
      click_button 'login'

      expect(page).to have_content 'no pipelines configured'
      expect(page.current_url).to match %r{\A#{ATC_URL}(\/?)\z}
    end
  end

  context 'when not logged in' do
    before(:each) do
      fly_login team_name
      fly('set-pipeline -n -p exposed-pipeline -c fixtures/resource-checking.yml')
      fly('unpause-pipeline -p exposed-pipeline')
      fly('expose-pipeline -p exposed-pipeline')
    end

    it 'redirects to login when triggering a new build' do
      visit dash_route("/teams/#{team_name}/pipelines/exposed-pipeline/jobs/checker")
      click_on 'Trigger Build'

      expect(page).to have_content 'login'

      fill_in 'login', with: ATC_USERNAME
      fill_in 'password', with: ATC_PASSWORD
      click_button 'login'

      expect(page.current_path).to include "/teams/#{team_name}/pipelines/exposed-pipeline/jobs/checker"
    end

    it 'redirects to login when pausing a resource' do
      visit dash_route("/teams/#{team_name}/pipelines/exposed-pipeline/resources/few-versions")
      click_on 'Pause Resource Checking'

      expect(page).to have_content 'login'

      fill_in 'login', with: ATC_USERNAME
      fill_in 'password', with: ATC_PASSWORD
      click_button 'login'

      expect(page.current_path).to include "/teams/#{team_name}/pipelines/exposed-pipeline/resources/few-versions"
    end
  end

  context 'when session expires' do
    it 'redirects to login from a non-exposed pipeline' do
      fly_login team_name
      fly('set-pipeline -n -p pipeline -c fixtures/simple-pipeline.yml')

      dash_login
      visit dash_route("/teams/#{team_name}/pipelines/pipeline")
      expect(page).to have_content ATC_USERNAME

      Capybara.current_session.driver.browser.manage.delete_all_cookies

      expect(page).to_not have_content ATC_USERNAME
      expect(page).to have_content 'password'
    end

    it 'changes top bar to prompt login from the dashboard' do
      dash_login
      visit dash_route
      expect(page).to have_content ATC_USERNAME

      Capybara.current_session.driver.browser.manage.delete_all_cookies

      expect(page).to_not have_content ATC_USERNAME
      expect(page).to have_content 'login'
    end
  end
end
