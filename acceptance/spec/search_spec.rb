require 'colors'

describe 'dashboard search', type: :feature do
  include Colors

  let!(:team_name) { generate_team_name }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    dash_login team_name

    fly('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p some-pipeline')

    fly('set-pipeline -n -p other-pipeline -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p other-pipeline')

    visit dash_route('/beta/dashboard')
  end

  it 'shows "no result" text when query returns nothing' do
    search 'invalid'
    expect(page).to have_content('No results for "invalid" matched your search.')
  end

  it 'clears the query by clicking the "x" button' do
    search 'some-text'
    expect(page.find_field('search-input-field').value).to eq 'some-text'

    page.find('.search-clear-button').click
    expect(page.find_field('search-input-field').value).to eq ''
  end

  context 'by pipeline name' do
    it 'filters the pipelines by the search term' do
      search 'some'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']

      fill_in 'search-input-field', with: ''
      search 'pipeline'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline', 'other-pipeline']
    end
  end

  describe 'by team name' do
    let!(:other_team_name) { generate_team_name }

    before do
      fly_login 'main'
      fly_with_input("set-team -n #{other_team_name} --no-really-i-dont-want-any-auth", 'y')

      fly_login other_team_name

      fly('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml')
      fly('unpause-pipeline -p some-pipeline')
      fly('expose-pipeline -p some-pipeline')

      fly('set-pipeline -n -p other-pipeline -c fixtures/states-pipeline.yml')
      fly('unpause-pipeline -p other-pipeline')
      fly('expose-pipeline -p other-pipeline')

      fly_login team_name
    end

    after do
      fly_login 'main'
      fly_with_input("destroy-team -n #{other_team_name}", other_team_name)
      fly('logout')
    end

    it 'filters the pipelines by team' do
      search "team:#{team_name}"
      expect(page.find_all('.dashboard-team-name').map(&:text)).to eq [team_name]

      fill_in 'search-input-field', with: ''
      search "team:#{team_name} some"
      expect(page.find_all('.dashboard-team-name').map(&:text)).to eq [team_name]
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end
  end

  context 'by pipeline status' do
    it 'filters the pipelines by succeeded status' do
      fly('trigger-job -w -j some-pipeline/passing')

      visit dash_route('/beta/dashboard')
      expect(border_palette).to eq(GREEN)

      search 'status:succeeded'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by errored status' do
      fly_fail('trigger-job -w -j some-pipeline/erroring')

      visit dash_route('/beta/dashboard')
      expect(border_palette).to eq(AMBER)

      search 'status:errored'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by aborted status' do
      fly('trigger-job -j some-pipeline/running')
      visit dash_route("/teams/#{team_name}/pipelines/some-pipeline/jobs/running/builds/1")
      expect(page).to have_content 'hello'

      fly('abort-build -j some-pipeline/running -b 1')
      visit dash_route('/beta/dashboard')
      expect(page).to have_css('.dashboard-pipeline.dashboard-status-aborted')
      expect(border_palette).to eq(BROWN)

      search 'status:aborted'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by paused status' do
      fly('pause-pipeline -p some-pipeline')

      visit dash_route('/beta/dashboard')
      expect(border_palette).to eq(BLUE)

      search 'status:paused'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by failed status' do
      fly_fail('trigger-job -w -j some-pipeline/failing')

      visit dash_route('/beta/dashboard')
      expect(border_palette).to eq(RED)

      search 'status:failed'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by pending status' do
      search 'status:pending'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline', 'other-pipeline']
    end

    it 'filters the pipelines by started status' do
      fly('trigger-job -j some-pipeline/running')
      visit dash_route("/teams/#{team_name}/pipelines/some-pipeline/jobs/running/builds/1")
      expect(page).to have_content 'hello'

      visit dash_route('/beta/dashboard')
      search 'status:running'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end
  end

  private

  def search(term)
    term.split('').each { |c| find_field('search-input-field').native.send_keys(c) }
  end

  def border_palette(pipeline = 'some-pipeline')
    background_palette(border_element(pipeline))
  end

  def border_element(pipeline = 'some-pipeline')
    page.find('.dashboard-pipeline', text: pipeline).find('.dashboard-pipeline-banner')
  end
end
