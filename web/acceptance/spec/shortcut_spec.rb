describe 'keyboard shortcut', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before(:each) do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    dash_login

    fly('set-pipeline -n -p pipeline -c fixtures/pipeline-with-long-output.yml')
    fly('unpause-pipeline -p pipeline')
  end

  describe 'when on builds page' do
    context 'pressing the "h" key' do
      it 'navigates to the next build' do
        fly('trigger-job -j pipeline/long-output')
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
        page.find('body').native.send_keys 'h'
        expect(page).to have_current_path("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/2")
      end
    end

    context 'pressing the "l" key' do
      it 'navigates to the previous build' do
        fly('trigger-job -j pipeline/long-output')
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/2")
        expect(page).to have_css '.steps'
        page.find('body').native.send_keys 'l'
        expect(page).to have_current_path("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
      end
    end

    context 'pressing the "j" key' do
      it 'scrolls down' do
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
        expect(page).to have_content('Line 100')
        scroll_to_top
        page.find('body').native.send_keys 'j'
        expect(scroll_position).to be > 0
      end
    end

    context 'pressing the "k" key' do
      it 'scrolls up' do
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
        expect(page).to have_content('Line 100')
        previous_scroll_position = scroll_position
        page.find('body').native.send_keys 'kkkkk'
        expect(scroll_position).to be < previous_scroll_position
      end
    end

    context 'pressing the "gg" key' do
      it 'scrolls to the top' do
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
        expect(page).to have_content('Line 100')
        page.find('body').native.send_keys 'gg'
        expect(scroll_position).to be 0
      end
    end

    context 'pressing the "G" key' do
      it 'scrolls to the bottom' do
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
        expect(page).to have_content('Line 100')
        scroll_to_top
        page.find('body').native.send_keys 'G'
        expect(scroll_position).to be > 0
      end
    end

    context 'pressing the "T" key' do
      it 'triggers a build' do
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
        page.find('body').native.send_keys 'T'
        expect { visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/2") }.to_not raise_error
      end
    end

    context 'pressing the "A" key' do
      it 'aborts a build' do
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
        page.find('body').native.send_keys 'A'
        expect(page).to have_content 'duration'
      end
    end

    context 'pressing the "?" key' do
      it 'shows the help' do
        fly('trigger-job -j pipeline/long-output')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output/builds/1")
        page.find('body').native.send_keys '?'
        expect(page).to have_content 'keyboard shortcuts'
      end
    end
  end

  describe 'when on dashboard page' do
    context 'pressing the "/" key' do
      it 'focus search input field' do
        visit dash_route
        page.find('body').native.send_keys '/'
        expect(page).to have_selector('#search-input-field:focus')
      end
    end
    context 'pressing the "?" key' do
      it 'shows the help' do
        visit dash_route
        page.find('body').native.send_keys '?'
        expect(page).to have_content 'keyboard shortcuts'
      end
    end
  end

  def scroll_position
    page.evaluate_script('window.scrollY')
  end

  def scroll_to_top
    page.evaluate_script('window.scrollTo(0, 0)')
  end
end
