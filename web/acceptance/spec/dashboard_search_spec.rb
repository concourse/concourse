require 'colors'

describe 'dashboard search', type: :feature do
  include Colors

  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    dash_login

    fly('set-pipeline -n -p some-pipeline -c fixtures/dashboard-pipeline.yml')
    fly('unpause-pipeline -p some-pipeline')

    fly('set-pipeline -n -p other-pipeline -c fixtures/dashboard-pipeline.yml')
    fly('unpause-pipeline -p other-pipeline')

    visit dash_route
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

  it 'keeps the search results with auto refresh' do
    expect(page).to have_content 'some-pipeline', wait: 1

    search 'invalid'
    expect(page).to have_content('No results for "invalid" matched your search.', wait: 1)

    sleep 5 # auto refresh interval
    expect(page).to have_content('No results for "invalid" matched your search.')
  end

  context 'by search query string' do
    it 'modifies the url to include the query' do
      search 'some-text'
      expect(page.current_url).to end_with '?search=some-text'
    end

    it 'cleans the url with empty query' do
      search 'some-text'
      search("\b" * 'some-text'.size)

      expect(page.current_url).to end_with '?'
    end

    it 'populates the query from the url' do
      visit dash_route('/dashboard?search=some-text')

      expect(page.find_field('search-input-field').value).to eq 'some-text'
    end
  end

  context 'by pipeline name' do
    it 'filters the pipelines by the search term' do
      search 'some'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']

      clear_search

      search 'pipeline'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline', 'other-pipeline']
    end
  end

  context 'by pipeline name with -' do
    it 'filters the pipelines by negate the search term' do
      search '-some'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['other-pipeline']

      clear_search

      search '-pipeline'
      expect(page).to have_content('No results for "-pipeline" matched your search.')
    end
  end

  describe 'by team name' do
    let!(:other_team_name) { generate_team_name }

    before do
      fly_login 'main'
      fly_with_input("set-team -n #{other_team_name} --local-user=#{ATC_USERNAME}", 'y')

      fly_login other_team_name

      fly('set-pipeline -n -p some-pipeline -c fixtures/dashboard-pipeline.yml')
      fly('unpause-pipeline -p some-pipeline')

      fly('set-pipeline -n -p other-pipeline -c fixtures/dashboard-pipeline.yml')
      fly('unpause-pipeline -p other-pipeline')

      dash_logout
      dash_login
      visit dash_route
    end

    after do
      fly_login 'main'
      fly_with_input("destroy-team -n #{other_team_name}", other_team_name)
      fly('logout')
    end

    it 'filters the pipelines by team' do
      search "team: #{team_name}"
      expect(page.find_all('.dashboard-team-name').map(&:text)).to eq [team_name]

      clear_search

      search "team: #{team_name} some"
      expect(page.find_all('.dashboard-team-name', minimum: 1).map(&:text)).to eq [team_name]
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']

      clear_search

      search "team: ma team: in"
      expect(page.find_all('.dashboard-team-name', minimum: 1).map(&:text)).to eq ["main"]
      expect(page).to have_content 'no pipelines set'
    end

    it 'filters the pipelines by negate team' do
      search "team: -#{team_name}"
      expect(page.find_all('.dashboard-team-name').map(&:text)).to eq ["main", other_team_name]

      clear_search

      search "team: -#{team_name} -some"
      expect(page.find_all('.dashboard-team-name').map(&:text)).to eq [other_team_name]
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['other-pipeline']
    end

    it 'filters the pipelines by negate team, negate pipeline and negate status' do
      search "team: -#{team_name}"
      expect(page.find_all('.dashboard-team-name').map(&:text)).to eq ["main", other_team_name]

      clear_search

      search "team: -#{team_name} team: -main status: -pending -some"
      expect(page).to have_content("No results for \"team: -#{team_name} team: -main status: -pending -some\" matched your search.")
    end
  end

  context 'by pipeline status' do
    it 'filters the pipelines by succeeded status' do
      fly('trigger-job -w -j some-pipeline/passing')

      visit dash_route
      expect(border_palette).to eq(GREEN)

      search 'status: succeeded'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by errored status' do
      fly_fail('trigger-job -w -j some-pipeline/erroring')

      visit dash_route
      expect(border_palette).to eq(AMBER)

      search 'status: errored'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by aborted status' do
      fly('trigger-job -j some-pipeline/running')
      visit dash_route("/teams/#{team_name}/pipelines/some-pipeline/jobs/running/builds/1")
      expect(page).to have_content 'hello'

      fly('abort-build -j some-pipeline/running -b 1')
      visit dash_route
      expect(page).not_to have_content 'running'
      expect(border_palette).to eq(BROWN)

      search 'status: aborted'
      expect(page.find_all('.dashboard-pipeline-name', minimum: 1).map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by paused status' do
      fly('pause-pipeline -p some-pipeline')

      visit dash_route
      expect(border_palette).to eq(BLUE)

      search 'status: paused'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by failed status' do
      fly_fail('trigger-job -w -j some-pipeline/failing')

      visit dash_route
      expect(border_palette).to eq(RED)

      search 'status: failed'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end

    it 'filters the pipelines by negate failed status' do
      fly_fail('trigger-job -w -j some-pipeline/failing')

      visit dash_route
      expect(border_palette).to eq(RED)

      search 'status: -failed'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['other-pipeline']
    end

    it 'filters the pipelines by pending status' do
      search 'status: pending'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline', 'other-pipeline']
    end

    it 'filters the pipelines by running status' do
      fly('trigger-job -j some-pipeline/running')
      visit dash_route("/teams/#{team_name}/pipelines/some-pipeline/jobs/running/builds/1")
      expect(page).to have_content 'hello'

      visit dash_route
      expect(page).to have_content 'some-pipeline'

      search 'status: running'
      expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq ['some-pipeline']
    end
  end

  private

  def clear_search
    page.find('#search-clear-button').click
    expect(page.find('#search-input-field').value).to be_empty
  end

  def search(term)
    page.find('#search-input-field').click
    sleep 0.5
    term.split('').each { |c| find_field('search-input-field').native.send_keys(c) }
  end

  def border_palette(pipeline = 'some-pipeline')
    background_palette(border_element(pipeline))
  end

  def border_element(pipeline = 'some-pipeline')
    page.find('.dashboard-pipeline', text: pipeline).find('.dashboard-pipeline-banner')
  end
end
