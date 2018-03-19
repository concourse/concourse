describe 'session', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home)  { Dir.mktmpdir }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')
  end

  context 'when not logged in' do
    before(:each) do
      fly_login team_name
      fly('set-pipeline -n -p test-pipeline -c fixtures/resource-checking.yml')
      fly('unpause-pipeline -p test-pipeline')
      fly('expose-pipeline -p test-pipeline')
    end

    it 'redirects to login when triggering a new build' do
      visit dash_route("/teams/#{team_name}/pipelines/test-pipeline/jobs/checker")
      click_on 'Trigger Build'
      expect(page).to have_current_path dash_route("/teams/#{team_name}/login?redirect=%2Fteams%2F#{team_name}%2Fpipelines%2Ftest-pipeline%2Fjobs%2Fchecker")
    end

    it 'redirects to login when pausing a resource' do
      visit dash_route("/teams/#{team_name}/pipelines/test-pipeline/resources/few-versions")
      click_on 'Pause Resource Checking'
      expect(page).to have_current_path dash_route("/teams/#{team_name}/login?redirect=%2Fteams%2F#{team_name}%2Fpipelines%2Ftest-pipeline%2Fresources%2Ffew-versions")
    end
  end

  xcontext 'when session expires' do
    it 'displays the correct state in the top bar' do
      dash_login
      visit dash_route
      expect(page).to have_content team_name

      within_window open_new_window do
        visit dash_route
        find('.user-info').click
        find('a', text: 'logout').click
      end

      expect(page).to_not have_content team_name
      expect(page).to have_content 'login'
    end

    it 'displays the correct state in the dashboard top bar' do
      dash_login
      visit dash_route('/dashboard')
      expect(page).to have_content team_name

      within_window open_new_window do
        visit dash_route
        find('.user-info').click
        find('a', text: 'logout').click
      end

      expect(page).to_not have_content team_name
      expect(page).to have_content 'login'
    end
  end
end
