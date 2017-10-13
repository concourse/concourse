require 'securerandom'

describe 'search', type: :feature do
  let(:main_team_name) { 'wats-team-main' }
  let(:test_team_name) { 'wats-team-test' }
  let(:status_team_name) { 'wats-team-status' }

  before(:each) do
    fly_with_input("set-team -n #{main_team_name} --no-really-i-dont-want-any-auth", 'y')
    fly_with_input("set-team -n #{test_team_name} --no-really-i-dont-want-any-auth", 'y')
    fly_with_input("set-team -n #{status_team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login main_team_name
    main_pipelines = %w[bosh main-team main-status]
    main_pipelines.each do |p|
      fly_with_input("destroy-pipeline -p #{p}-status-pipeline", 'y')
      fly("set-pipeline -n -p #{p}-pipeline -c fixtures/states-pipeline.yml")
      fly("unpause-pipeline -p #{p}-pipeline")
      fly("expose-pipeline -p #{p}-pipeline")
    end

    fly_login test_team_name
    test_pipelines = %w[maintenance test-team test-status]
    test_pipelines.each do |p|
      fly_with_input("destroy-pipeline -p #{p}-status-pipeline", 'y')
      fly("set-pipeline -n -p #{p}-pipeline -c fixtures/states-pipeline.yml")
      fly("unpause-pipeline -p #{p}-pipeline")
      fly("expose-pipeline -p #{p}-pipeline")
    end
  end

  describe 'pipelines' do
    it 'returns pipeline names that match the search term' do
      search_by_query('main-pipeline')
      within '.dashboard-team-group', text: main_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['main-team-pipeline', 'main-status-pipeline']
        )
      end
      within '.dashboard-team-group', text: test_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['maintenance-pipeline']
        )
      end
    end

    it 'returns pipeline names that contain word team in the search term' do
      search_by_query('team')
      within '.dashboard-team-group', text: main_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['main-team-pipeline']
        )
      end
      within '.dashboard-team-group', text: test_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['test-team-pipeline']
        )
      end
    end

    it 'returns pipeline names that contain word status in the search term' do
      search_by_query('status')
      within '.dashboard-team-group', text: main_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['main-status-pipeline']
        )
      end
      within '.dashboard-team-group', text: test_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['test-status-pipeline']
        )
      end
    end

    it 'returns no pipelines name when it does not match the search term' do
      search_by_query('unknown')
      within '.dashboard-content' do
        expect(page).to have_content('No results for "unknown" matched your search.')
      end
    end
  end

  describe 'teams' do
    it 'returns team names that match the search term' do
      search_by_query('team:main')
      within '.dashboard-team-group', text: main_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['bosh-pipeline', 'main-team-pipeline', 'main-status-pipeline']
        )
      end
    end

    it 'returns pipelines by team name that match the search term' do
      search_by_query('team:main bosh')
      within '.dashboard-team-group', text: main_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['bosh-pipeline']
        )
      end
      expect(page).to_not have_content('wats-team-test')
    end

    it 'returns no teams when it does not match the search term' do
      search_by_query('team:kubo')
      within '.dashboard-content' do
        expect(page).to have_content('No results for "team:kubo" matched your search.')
      end
    end

    context 'clear search' do
      it 'clears the search input field' do
        search_by_query('main')
        page.find('.search-clear-button').click
        expect(page.find_by_id('search-input-field').text).to eq ''
      end
    end
  end

  describe 'status' do
    status_names = %w[succeeded errored aborted paused failed pending started]
    before(:each) do
      fly_login status_team_name
      status_names.each do |p|
        fly("set-pipeline -n -p #{p}-status-pipeline -c fixtures/states-pipeline.yml")
        fly("unpause-pipeline -p #{p}-status-pipeline")
        fly("expose-pipeline -p #{p}-status-pipeline")
      end
      fly_login status_team_name
    end

    after(:each) do
      fly_login status_team_name
      status_names.each do |p|
        fly_with_input("destroy-pipeline -p #{p}-status-pipeline", 'y')
      end
    end

    it 'returns pipelines by status succeeded' do
      fly('trigger-job -w -j succeeded-status-pipeline/passing')
      sleep 10
      search_by_query('status:succeeded')
      within '.dashboard-team-group', text: status_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['succeeded-status-pipeline']
        )
      end
    end

    it 'returns pipelines by status errored' do
      fly_fail('trigger-job -w -j errored-status-pipeline/erroring')
      sleep 10
      search_by_query('status:error')
      within '.dashboard-team-group', text: status_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['errored-status-pipeline']
        )
      end
    end

    it 'returns pipelines by status aborted' do
      fly('trigger-job -j aborted-status-pipeline/hanging')
      fly('abort-build -j aborted-status-pipeline/hanging -b 1')
      sleep 10
      search_by_query('status:abort')
      within '.dashboard-team-group', text: status_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['aborted-status-pipeline']
        )
      end
    end

    it 'returns pipelines by status paused' do
      fly('pause-pipeline -p paused-status-pipeline')
      sleep 10
      search_by_query('status:pause')
      within '.dashboard-team-group', text: status_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['paused-status-pipeline']
        )
      end
    end

    it 'returns pipelines by status failed' do
      fly_fail('trigger-job -w -j failed-status-pipeline/failing')
      sleep 10
      search_by_query('status:fail')
      within '.dashboard-team-group', text: status_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['failed-status-pipeline']
        )
      end
    end

    it 'returns pipelines by status pending' do
      search_by_query('status:pending')
      within '.dashboard-team-group', text: status_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['succeeded-status-pipeline', 'errored-status-pipeline', 'aborted-status-pipeline', 'paused-status-pipeline', 'failed-status-pipeline', 'pending-status-pipeline', 'started-status-pipeline']
        )
      end
    end

    # TODO: handle pipeline running
    xit 'returns pipelines by status started' do
      fly('trigger-job -j started-status-pipeline/running')
      search_by_query('status:start')
      within '.dashboard-team-group', text: status_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['started-status-pipeline']
        )
      end
    end

    it 'returns pipelines by status and by team names that match the search term' do
      fly_fail('trigger-job -w -j failed-status-pipeline/failing')
      sleep 10
      search_by_query('status:fail team:status')
      within '.dashboard-team-group', text: status_team_name do
        expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
          ['failed-status-pipeline']
        )
      end
      expect(page).to_not have_content('wats-team-main')
      expect(page).to_not have_content('wats-team-test')
    end
  end

  private

  def search_by_query(query, element = 'search-input-field')
    visit dash_route('/dashboard')
    query.split('').map do |k|
      sleep 0.2
      page.find_by_id(element).send_keys k.to_s
    end
    expect(page.find_field(element).value).to eq query.to_s
  end
end
