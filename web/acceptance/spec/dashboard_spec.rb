require 'securerandom'

describe 'dashboard', type: :feature do
  let(:team_name) { generate_team_name }
  let(:other_team_name) { generate_team_name }

  before(:each) do
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')
    fly_with_input("set-team -n #{other_team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    fly('set-pipeline -n -p some-pipeline -c fixtures/test-pipeline.yml')

    fly_login other_team_name
    fly('set-pipeline -n -p other-pipeline-private -c fixtures/test-pipeline.yml')
    fly('set-pipeline -n -p other-pipeline-public -c fixtures/test-pipeline.yml')
    fly('expose-pipeline -p other-pipeline-public')

    fly_login team_name
  end

  it 'shows all pipelins from the authenticated team and public pipelines from other teams' do
    dash_login team_name

    visit dash_route('/dashboard')

    within '.dashboard-team-group', text: team_name do
      expect(page).to have_content 'some-pipeline'
    end

    within '.dashboard-team-group', text: other_team_name do
      expect(page).to have_content 'other-pipeline-public'
      expect(page).to_not have_content 'other-pipeline-private'
    end
  end
end
