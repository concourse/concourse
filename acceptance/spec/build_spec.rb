require 'colors'

describe 'build', type: :feature do
  include Colors

  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    dash_login
  end

  describe 'long build logs', :perf do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/pipeline-with-10k-output.yml')
      fly('unpause-pipeline -p pipeline')
      fly('trigger-job -w -j pipeline/10k-output')
      visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/10k-output/builds/1")
    end

    context 'when output has 10k lines' do
      it 'should load the page' do
        page.find('.build-step .header', text: 'print').click
        expect(page).to have_content('Line 10000', wait: 20)
      end
    end
  end

  describe 'step arrow color' do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/resource-checking.yml')
      fly('unpause-pipeline -p pipeline')
      fly('trigger-job -w -j pipeline/checker')
      visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/checker/builds/1")
    end

    context "new resource version found" do
      it "set step arrow color to yellow" do
        expect(foreground_color(page.find('.build-step[data-step-name="few-versions"]').find('.fa-arrow-down'))).to eq(YELLOW)
      end
    end

    context "no new resource version" do
      it "does not change step arrow color " do
        fly('trigger-job -w -j pipeline/checker')
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/checker/builds/2")

        expect(foreground_color(page.find('.build-step[data-step-name="few-versions"]').find('.fa-arrow-down'))).to eq(WHITE)
      end
    end
  end

  describe 'build logs' do
    let(:timestamp_regex) { /\d{2}:\d{2}:\d{2}/ }

    def timestamp_for_line(line)
      page.evaluate_script "window.getComputedStyle($('.timestamp')[#{line}], ':before').getPropertyValue('content')"
    end

    def timestamp_color_for_line(line)
      color = page.evaluate_script "window.getComputedStyle($('.timestamp')[#{line}], ':before').getPropertyValue('color')"
      by_rgb(color).closest_match(PALETTE)
    end

    before do
      fly('set-pipeline -n -p pipeline -c fixtures/pipeline-with-long-output.yml')
      fly('unpause-pipeline -p pipeline')
      fly('trigger-job -w -j pipeline/long-output-passing')

      visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/long-output-passing/builds/1")
      expect(page).to_not have_content 'Line 5'

      page.find('.build-step .header', text: 'print').click
      within '.steps' do
        expect(timestamp_for_line(1)).to match(timestamp_regex)
      end
    end

    it 'has linkable timestamps for each line' do
      timestamp = page.all('.timestamp')[5]
      timestamp.click
      within '.steps' do
        expect(timestamp_color_for_line(5)).to eq(AMBER)
      end

      # visit the URL to show that the link's target link w/ anchor element works
      # by expanding the step to reveal the line and highlighting the line
      visit current_url
      expect(page).to have_content 'Line 5'
      within '.steps' do
        expect(timestamp_color_for_line(5)).to eq(AMBER)
      end
    end

    it 'has range-linkable timestamps for each line' do
      first_timestamp = page.all('.timestamp')[2]
      last_timestamp = page.all('.timestamp')[7]

      first_timestamp.click
      page.driver.browser.action.key_down(:shift).click(last_timestamp.native).key_up(:shift).perform
      within '.steps' do
        (2..7).each do |line_number|
          expect(timestamp_color_for_line(line_number)).to eq(AMBER)
        end
      end

      visit current_url
      within '.steps' do
        (2..7).each do |line_number|
          expect(page).to have_content "Line #{line_number}"
          expect(timestamp_color_for_line(line_number)).to eq(AMBER)
        end
      end
    end
  end

  describe 'builds in different states' do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/states-pipeline.yml')
      fly('unpause-pipeline -p pipeline')
    end

    context 'failed' do
      before do
        fly_fail('trigger-job -w -j pipeline/failing')
      end

      it 'shows the build output for failed steps' do
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/failing/builds/1")
        expect(page).to have_content 'i failed'
        expect(background_palette(page.find('.build-header'))).to eq(RED)
        expect(background_palette(page.find('#builds .current'))).to eq(RED)
      end
    end

    context 'succeeded' do
      before do
        fly('trigger-job -w -j pipeline/passing')
      end

      it 'hides the build output for successful steps, until toggled' do
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/passing/builds/1")
        expect(page).to_not have_content 'i passed'
        page.find('.build-step .header', text: 'pass').click
        expect(page).to have_content 'i passed'
        expect(background_palette(page.find('.build-header'))).to eq(GREEN)
        expect(background_palette(page.find('#builds .current'))).to eq(GREEN)
      end
    end

    context 'when a build is running' do
      before do
        fly('trigger-job -j pipeline/running')
      end

      it 'can be aborted' do
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/running/builds/1")
        expect(page).to have_button 'Abort Build'
        page.find_button('Abort Build').click
        fly_fail('watch -j pipeline/running')
        within '.step-body' do
          expect(page).to have_content 'interrupted'
        end
        expect(background_palette(page.find('.build-header'))).to eq(BROWN)
        expect(background_palette(page.find('#builds .current'))).to eq(BROWN)
      end
    end

    context 'when a build is pending' do
      it 'pinned version is unavailable' do
        job_name = 'unavailable-pinned-input'
        fly("trigger-job -j pipeline/#{job_name}")
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/#{job_name}/builds/1")
        expect(page).to have_content 'pinned version {"time":"2017-08-11T00:13:33.123805549Z"} is not available'
        expect(background_color(page.find('.build-header'))).to be_greyscale
        expect(background_color(page.find('#builds .current'))).to be_greyscale
      end

      it 'no version available' do
        job_name = 'pending'
        fly("trigger-job -j pipeline/#{job_name}")
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/#{job_name}/builds/1")
        expect(page).to have_content 'no versions available'
        expect(background_color(page.find('.build-header'))).to be_greyscale
        expect(background_color(page.find('#builds .current'))).to be_greyscale
      end

      it 'no versions have passed constraints' do
        job_name = 'unavailable-constrained-input'
        fly("trigger-job -j pipeline/#{job_name}")
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/#{job_name}/builds/1")
        expect(page).to have_content 'no versions satisfy passed constraints'
        expect(background_color(page.find('.build-header'))).to be_greyscale
        expect(background_color(page.find('#builds .current'))).to be_greyscale
      end
    end
  end

  context 'when a job has manual triggering enabled' do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/manual-trigger-enabled.yml')
      fly('unpause-pipeline -p pipeline')
    end

    it 'can be manually triggered' do
      visit dash_route("/teams/#{team_name}/pipelines/pipeline")
      expect(page).to have_content 'manual-trigger'

      page.find('a', text: 'manual-trigger').click
      page.find_button('Trigger Build').click
      expect(page).to have_content 'manual-trigger #1'
    end

    context 'when manual triggering is disabled' do
      before do
        fly('trigger-job -w -j pipeline/manual-trigger')
        fly('set-pipeline -n -p pipeline -c fixtures/manual-trigger-disabled.yml')
      end

      it 'cannot be manually triggered from the job page' do
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/manual-trigger")
        expect(page.find('.build-action')).to be_disabled

        page.find_button('Trigger Build', disabled: true).click
        expect(page).to_not have_content 'manual-trigger #2'
      end

      it 'cannot be manually triggered from the build page' do
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/manual-trigger/builds/1")
        page.find_button('Trigger Build', disabled: true).click
        expect(page).to_not have_content 'manual-trigger #2'
      end
    end
  end
end
