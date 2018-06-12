describe 'pipeline', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before(:each) do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    fly('set-pipeline -n -p test-pipeline -c fixtures/pipeline-with-slashes.yml')

    dash_login
  end

  context 'with jobs and resources that have unescaped names' do
    it 'displays the unescaped names in the pipeline view' do
      expect(page.find('.job')).to have_content 'some/job'
      expect(page.find('.input')).to have_content 'some/resource'
    end

    it 'can navigate to the escaped links' do
      page.find('a > text', text: 'some/job').click
      expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline/jobs/some%2Fjob"

      page.go_back

      expect(page).to have_content 'some/resource'
      page.find('a > text', text: 'some/resource').click
      expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline/resources/some%2Fresource"
    end

    context 'with builds triggered' do
      it 'can navigate to the build of escaped links of job name' do
        fly('unpause-pipeline -p test-pipeline')
        fly('trigger-job -j test-pipeline/some/job')
        dash_login

        page.find('a > text', text: 'some/job').click
        expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline/jobs/some%2Fjob/builds/1"
      end
    end
  end

  it 'is linked in the sidebar' do
    page.find('.sidebar-toggle').click

    within('.sidebar') do
      expect(page).to have_content('test-pipeline')
      expect(page).to have_link('test-pipeline')

      click_on 'test-pipeline'
      expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline"
    end
  end

  it 'can be unpaused' do
    page.find('.sidebar-toggle').click

    pause_button = page.find('.draggable', text: 'test-pipeline').find('.btn-pause')
    expect(pause_button['class']).to include 'enabled'
    expect(page).to have_css('.top-bar.paused')

    pause_button.click
    expect(page).to_not have_css('.top-bar.paused')
    expect(pause_button['class']).to include 'disabled'
  end

  it 'can be paused' do
    fly('unpause-pipeline -p test-pipeline')
    visit dash_route

    page.find('.sidebar-toggle').click

    pause_button = page.find('.draggable', text: 'test-pipeline').find('.btn-pause')
    expect(pause_button['class']).to include 'disabled'
    expect(page).to_not have_css('.top-bar.paused')

    pause_button.click
    expect(page).to have_css('.top-bar.paused')
    expect(pause_button['class']).to include 'enabled'
  end
end
