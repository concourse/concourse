describe 'session', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home)  { Dir.mktmpdir }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')
  end

  context 'when logging into a team with space in its name' do
    it 'displays the team name correctly' do
      fly_with_input("set-team -n \"#{team_name} test\" --no-really-i-dont-want-any-auth", 'y')

      visit dash_route("/teams/#{team_name}%20test/login")
      expect(page).to have_content "logging in to #{team_name} test"

      fly_with_input("destroy-team -n \"#{team_name} test\"", "#{team_name} test")
    end
  end

  xcontext 'when session expires' do
    it 'displays the correct state in the top bar' do
      dash_login team_name
      visit dash_route('/beta/dashboard')
      expect(page).to have_content team_name

      within_window open_new_window do
        visit dash_route
        find('.user-info').click
        find('a', text: 'logout').click
      end

      expect(page).to_not have_content team_name
      expect(page).to have_content 'log in'
    end
  end
end
