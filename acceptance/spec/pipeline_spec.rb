describe 'pipeline', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before(:each) do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    fly('set-pipeline -n -p test-pipeline -c fixtures/pipeline.yml')

    dash_login
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
