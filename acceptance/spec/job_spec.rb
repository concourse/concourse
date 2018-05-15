describe 'job', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --allow-all-users", 'y')

    fly_login team_name
    fly('set-pipeline -n -p test-pipeline -c fixtures/passing-pipeline.yml')
    fly('unpause-pipeline -p test-pipeline')

    dash_login
  end

  context 'without builds' do
    it 'links to the builds page' do
      page.find('a > text', text: 'passing').click
      expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline/jobs/passing"
    end
  end

  context 'with builds' do
    before do
      fly('trigger-job -w -j test-pipeline/passing')
      visit dash_route
    end

    it 'links to the latest build' do
      page.find('a > text', text: 'passing').click
      expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline/jobs/passing/builds/1"
      click_on 'passing #1'
      expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline/jobs/passing"
    end
  end

  it 'can be paused' do
    fly('unpause-job -j test-pipeline/passing')
    visit dash_route("/teams/#{team_name}/pipelines/test-pipeline/jobs/passing")

    page.find_by_id('job-state').click
    pause_button = page.find_by_id('job-state')
    sleep 5
    expect(pause_button['class']).to include 'enabled'
    expect(pause_button['class']).to_not include 'disabled'

    visit dash_route("/teams/#{team_name}/pipelines/test-pipeline")
    expect(page).to have_css('.job.paused', text: 'passing')
  end

  it 'can be unpaused' do
    fly('pause-job -j test-pipeline/passing')
    visit dash_route("/teams/#{team_name}/pipelines/test-pipeline/jobs/passing")

    page.find_by_id('job-state').click
    pause_button = page.find_by_id('job-state')

    expect(pause_button['class']).to_not include 'enabled'
    expect(pause_button['class']).to include 'disabled'
  end
end
