require 'securerandom'
require 'colors'

describe 'dashboard', type: :feature do
  include Colors

  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }
  let(:username) { ATC_USERNAME }

  before(:each) do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')
    fly_login team_name
    fly('set-pipeline -n -p some-pipeline -c fixtures/dashboard-pipeline.yml')
    fly('unpause-pipeline -p some-pipeline')
  end

  describe 'view toggle' do
    context 'when the view is the default view' do
      it 'switches to compact view' do
        dash_login
        visit dash_route('/dashboard')
        expect(page).to have_content('HIGH-DENSITY')

        click_on 'high-density'
        expect(page).to have_current_path '/dashboard/hd'
      end
    end

    context 'when the view is the compact view' do
      it 'switches to default view' do
        dash_login
        visit dash_route('/dashboard/hd')
        expect(page).to have_content('HIGH-DENSITY')

        click_on 'high-density'
        expect(page).to have_current_path '/dashboard'
      end
    end
  end

  describe 'default view' do
    context 'with no user logged in' do
      it 'displays a login button' do
        visit dash_route('/dashboard')
        expect(page).to have_link('login', href: '/sky/login')
      end
    end

    context 'with multiple teams' do
      let(:other_team_name) { generate_team_name }

      before do
        fly_login 'main'
        fly_with_input("set-team -n #{other_team_name} --local-user=#{ATC_USERNAME}", 'y')

        fly_login other_team_name
        fly('set-pipeline -n -p other-pipeline-private -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p other-pipeline-private')
        fly('set-pipeline -n -p other-pipeline-public -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p other-pipeline-public')
        fly('expose-pipeline -p other-pipeline-public')

        fly_with_input("set-team -n #{other_team_name} --local-user=bad-username", 'y')
        fly_login team_name
      end

      after do
        fly_login 'main'
        fly_with_input("destroy-team -n #{other_team_name}", other_team_name)
      end

      it 'shows all pipelines from the authenticated team and public pipelines from other teams' do
        dash_login
        visit dash_route('/dashboard')
        within '.dashboard-team-group', text: team_name do
          expect(page).to have_content 'some-pipeline'
        end

        within '.dashboard-team-group', text: other_team_name do
          expect(page).to have_content 'other-pipeline-public'
          expect(page).to_not have_content 'other-pipeline-private'
        end
      end

      it 'shows authenticated team first' do
        dash_login

        visit dash_route('/dashboard')

        expect(page).to have_content(team_name)
        expect(page).to have_content(other_team_name)
        expect(page.first('.dashboard-team-name').text).to eq(team_name)
      end
    end

    context 'when pipelines have different states' do
      before do
        fly('destroy-pipeline -n -p some-pipeline')

        fly('set-pipeline -n -p failing-pipeline -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p failing-pipeline')
        fly_fail('trigger-job -w -j failing-pipeline/failing')

        fly('set-pipeline -n -p other-failing-pipeline -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p other-failing-pipeline')
        fly_fail('trigger-job -w -j other-failing-pipeline/failing')
        fly('trigger-job -j other-failing-pipeline/running')

        fly('set-pipeline -n -p errored-pipeline -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p errored-pipeline')
        fly_fail('trigger-job -w -j errored-pipeline/erroring')

        fly('set-pipeline -n -p aborted-pipeline -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p aborted-pipeline')
        fly('trigger-job -j aborted-pipeline/running')
        fly('abort-build -j aborted-pipeline/running -b 1')

        fly('set-pipeline -n -p paused-pipeline -c fixtures/dashboard-pipeline.yml')

        fly('set-pipeline -n -p succeeded-pipeline -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p succeeded-pipeline')
        fly('trigger-job -w -j succeeded-pipeline/passing')

        fly('set-pipeline -n -p pending-pipeline -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p pending-pipeline')
      end

      it 'displays the pipelines in correct sort order' do
        visit_dashboard
        within '.dashboard-team-group', text: team_name do
          expect(page.find_all('.dashboard-pipeline-name').map(&:text)).to eq(
            ['failing-pipeline', 'other-failing-pipeline', 'errored-pipeline', 'aborted-pipeline', 'paused-pipeline', 'succeeded-pipeline', 'pending-pipeline']
          )
        end
      end
    end

    context 'when a pipeline is paused' do
      before do
        fly('pause-pipeline -p some-pipeline')
        visit_dashboard
      end

      it 'is shown in blue' do
        expect(banner_palette).to eq(BLUE)
      end

      it 'is labelled "paused"' do
        within '.dashboard-pipeline', text: 'some-pipeline' do
          expect(page).to have_content('paused')
        end
      end
    end

    context 'when a pipeline is pending' do
      before do
        fly('trigger-job -j some-pipeline/pending')
        visit_dashboard
      end

      it 'is shown in grey' do
        expect(banner_color).to be_greyscale
      end

      it 'is labelled "pending"' do
        within '.dashboard-pipeline', text: 'some-pipeline' do
          expect(page).to have_content('pending', wait: 10)
        end
      end
    end

    context 'when a pipeline has a failed build' do
      before(:each) do
        fly('set-pipeline -n -p some-other-pipeline -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p some-other-pipeline')
        fly_fail('trigger-job -w -j some-other-pipeline/failing')
      end

      it 'is shown in red' do
        visit_dashboard
        expect(banner_palette('some-other-pipeline')).to eq(RED)
      end
    end

    context 'when a pipeline has a passed build' do
      before do
        fly('trigger-job -w -j some-pipeline/passing')
      end

      it 'is shown in green' do
        visit_dashboard
        expect(banner_palette).to eq(GREEN)
      end
    end

    context 'when a pipeline has an aborted build' do
      before do
        fly('trigger-job -j some-pipeline/running')
        visit_dashboard
        expect(page).to have_css('.dashboard-pipeline.dashboard-running')

        fly('abort-build -j some-pipeline/running -b 1')
      end

      it 'is shown in brown' do
        expect(page).to have_css('.dashboard-pipeline.dashboard-status-aborted')
        expect(banner_palette).to eq(BROWN)
      end
    end

    context 'when a pipeline has no builds' do
      it 'is shown in grey' do
        visit_dashboard
        expect(banner_color).to be_greyscale
      end
    end

    context 'when a pipeline has an errored build' do
      before do
        fly_fail('trigger-job -w -j some-pipeline/erroring')
      end

      it 'is shown in amber' do
        visit_dashboard
        expect(banner_palette).to eq(AMBER)
      end
    end

    context 'when a pipeline changes its state' do
      it 'updates the dashboard automatically' do
        visit_dashboard
        expect(banner_color).to be_greyscale
        fly('trigger-job -w -j some-pipeline/passing')
        sleep 5
        expect(banner_palette).to eq(GREEN)
      end

      it 'shows the time since last state change' do
        visit_dashboard

        fly('trigger-job -w -j some-pipeline/passing')
        sleep 5
        duration1 = page.text.match(/some-pipeline\n([\d]{1,2})/)[1].to_i

        fly('trigger-job -w -j some-pipeline/passing')
        sleep 5
        duration2 = page.text.match(/some-pipeline\n([\d]{1,2})/)[1].to_i

        expect(duration2).to be > duration1

        fly_fail('trigger-job -w -j some-pipeline/failing')
        sleep 5
        duration3 = page.text.match(/some-pipeline\n([\d]{1,2})/)[1].to_i

        expect(duration3).to be < duration2
      end
    end

    context 'when a pipeline has one or more resources errored' do
      it 'shows resource error indicator' do
        fly_fail('check-resource -r some-pipeline/broken-time')

        visit_dashboard
        within('.dashboard-pipeline-header', text: 'some-pipeline') do
          expect(page).to have_css('.dashboard-resource-error')
        end
      end
    end

    it 'anchors URL links on team groups' do
      visit_dashboard
      expect(page).to have_css('.dashboard-team-group', id: team_name)
    end

    it 'links to latest build in the preview' do
      fly_fail('trigger-job -w -j some-pipeline/failing')
      visit_dashboard
      expect(page).to have_css("a[href=\"/teams/#{team_name}/pipelines/some-pipeline/jobs/failing/builds/1\"]")
      expect(page.find("a[href=\"/teams/#{team_name}/pipelines/some-pipeline/jobs/failing/builds/1\"]").text).not_to be_nil
    end
  end

  describe 'logout' do
    context 'with user logged in' do
      before(:each) do
        fly('set-pipeline -n -p some-other-pipeline -c fixtures/dashboard-pipeline.yml')
        fly('expose-pipeline -p some-other-pipeline')
      end

      it 'logout current user' do
        visit_dashboard

        expect(page).to have_content ATC_USERNAME.to_s
        expect(page).to have_content 'some-pipeline'
        expect(page).to have_content 'some-other-pipeline'

        page.find('.user-id', text: username).click
        expect(page).to have_content 'logout'

        page.find('.user-menu', text: 'logout').click

        expect(page).to_not have_content 'logout'
        expect(page).to have_content 'login'

        expect(page).to have_content 'some-other-pipeline'
        expect(page).to_not have_content 'some-pipeline'
      end
    end
  end

  describe 'high density view' do
    context 'with no user logged in' do
      it 'displays a login button' do
        visit dash_route('/dashboard/hd')
        expect(page).to have_link('login', href: '/sky/login')
      end
    end

    context 'when a pipeline is paused' do
      before do
        fly('pause-pipeline -p some-pipeline')
        visit_hd_dashboard
      end

      it 'has a blue banner' do
        expect(banner_palette).to eq(BLUE)
      end

      it 'displays the name with a black background' do
        expect(title_palette).to eq(BLACK)
      end
    end

    context 'when a pipeline is pending' do
      before do
        fly('trigger-job -j some-pipeline/pending')
        visit_hd_dashboard
      end

      it 'has a gray banner' do
        expect(banner_color).to be_greyscale
      end

      it 'displays its name with a black background' do
        expect(title_palette).to eq(BLACK)
      end
    end

    context 'when a pipeline has a failed build' do
      before(:each) do
        fly_fail('trigger-job -w -j some-pipeline/failing')
        visit_hd_dashboard
      end

      it 'has a red banner' do
        expect(banner_palette).to eq(RED)
      end

      it 'displays its name with a red background' do
        expect(title_palette).to eq(RED)
      end
    end

    context 'when a pipeline has a passed build' do
      before do
        fly('trigger-job -w -j some-pipeline/passing')
        visit_hd_dashboard
      end

      it 'has a green banner' do
        expect(banner_palette).to eq(GREEN)
      end

      it 'displays its name in a black background' do
        expect(title_palette).to eq(GREEN)
      end
    end

    context 'when a pipeline has an aborted build' do
      before do
        fly('trigger-job -j some-pipeline/running')
        visit_hd_dashboard
        expect(page).to have_css('.dashboard-pipeline.dashboard-running')

        fly('abort-build -j some-pipeline/running -b 1')
      end

      it 'has a brown banner' do
        expect(page).to have_css('.dashboard-pipeline.dashboard-status-aborted')
        expect(banner_palette).to eq(BROWN)
      end

      it 'displays its name in a black background' do
        expect(title_palette).to eq(BLACK)
      end
    end

    context 'when a pipeline has no builds' do
      before do
        visit_hd_dashboard
      end

      it 'has a gray banner' do
        expect(banner_color).to be_greyscale
      end

      it 'displays its name in a black background' do
        expect(title_palette).to eq(BLACK)
      end
    end

    context 'when a pipeline has an errored build' do
      before do
        fly_fail('trigger-job -w -j some-pipeline/erroring')
        visit_hd_dashboard
      end

      it 'has an amber banner' do
        expect(banner_palette).to eq(AMBER)
      end

      it 'displays its name in an amber background' do
        expect(title_palette).to eq(AMBER)
      end
    end

    context 'when a pipeline changes its state' do
      it 'updates the dashboard automatically' do
        visit_hd_dashboard
        expect(banner_color).to be_greyscale
        fly('trigger-job -w -j some-pipeline/passing')
        sleep 5
        expect(banner_palette).to eq(GREEN)
      end
    end

    context 'with multiple teams' do
      let(:other_team_name) { generate_team_name }

      before do
        fly_login 'main'
        fly_with_input("set-team -n #{other_team_name} --local-user=#{ATC_USERNAME}", 'y')

        fly_login other_team_name
        fly('set-pipeline -n -p other-pipeline-private -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p other-pipeline-private')
        fly('set-pipeline -n -p other-pipeline-public -c fixtures/dashboard-pipeline.yml')
        fly('unpause-pipeline -p other-pipeline-public')
        fly('expose-pipeline -p other-pipeline-public')

        fly_with_input("set-team -n #{other_team_name} --local-user=bad-username", 'y')
        fly_login team_name
      end

      after do
        fly_login 'main'
        fly_with_input("destroy-team -n #{other_team_name}", other_team_name)
      end

      it 'shows all pipelines from the authenticated team and public pipelines from other teams' do
        visit_hd_dashboard
        expect(page).to have_content 'some-pipeline'
        expect(page).to have_content 'other-pipeline-public'
        expect(page).to_not have_content 'other-pipeline-private'
      end

      it 'shows the teams ordered by the number of pipelines' do
        fly_login 'main'
        fly_with_input("set-team -n #{other_team_name} --local-user=#{ATC_USERNAME}", 'y')

        fly_login other_team_name
        fly('expose-pipeline -p other-pipeline-private')

        fly_with_input("set-team -n #{other_team_name} --local-user=bad-username", 'y')
        fly_login team_name

        visit dash_route('/dashboard/hd')
        expect(page).to have_css('.dashboard-team-name')
        expect(page.first('.dashboard-team-name').text).to eq(other_team_name)

        dash_login
        visit_hd_dashboard
        expect(page).to have_content(team_name)
        expect(page).to have_content(other_team_name)
        expect(page.first('.dashboard-team-name').text).to eq(team_name)
      end
    end
  end

  private

  def login
    @login ||= dash_login
  end

  def banner_palette(pipeline = 'some-pipeline')
    background_palette(banner_element(pipeline))
  end

  def banner_color(pipeline = 'some-pipeline')
    background_color(banner_element(pipeline))
  end

  def banner_element(pipeline = 'some-pipeline')
    page.find('.dashboard-pipeline', text: pipeline).find('.dashboard-pipeline-banner')
  end

  def title_palette(pipeline = 'some-pipeline')
    background_palette(title_element(pipeline))
  end

  def title_element(pipeline = 'some-pipeline')
    page.find('.dashboard-pipeline-content', text: pipeline)
  end

  def visit_dashboard
    login
    visit dash_route('/dashboard')
  end

  def visit_hd_dashboard
    login
    visit dash_route('/dashboard/hd')
  end
end
