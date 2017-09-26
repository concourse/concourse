require 'colors'

describe 'build', type: :feature do
  include Colors

  let(:team_name) { generate_team_name }

  before(:each) do
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    dash_login team_name
  end

  context 'when a build is running' do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/states-pipeline.yml')
      fly('unpause-pipeline -p pipeline')
      fly('trigger-job -j pipeline/running')
    end

    it 'can be aborted' do
      visit dash_route("/teams/#{team_name}/pipelines/pipeline")
      page.find('a', text: 'running').click
      page.find_button("Abort Build").click
      fly_fail('watch -j pipeline/running')
      expect(page).to have_content 'interrupted'
      expect(background_palette(page.find('.build-header'))).to eq(BROWN)
      expect(background_palette(page.find('#builds .current'))).to eq(BROWN)
    end
  end

  context 'when a job has manual triggering enabled' do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/manual-trigger-enabled.yml')
      fly('unpause-pipeline -p pipeline')
    end

    it 'can be manually triggered' do
      visit dash_route("/teams/#{team_name}")
      page.find('a', text: 'manual-trigger').click
      page.find_button("Trigger Build").click
      expect(page).to have_content "manual-trigger #1"
    end

    context 'when manual triggering is disabled' do
      before do
        fly('trigger-job -w -j pipeline/manual-trigger')
        fly('set-pipeline -n -p pipeline -c fixtures/manual-trigger-disabled.yml')
      end

      it 'cannot be manually triggered from the job page' do
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/manual-trigger")
        page.find_button("Trigger Build", disabled: true).click
        expect(page).to_not have_content "manual-trigger #2"
      end

      it 'cannot be manually triggered from the build page' do
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/jobs/manual-trigger/builds/1")
        page.find_button("Trigger Build", disabled: true).click
        expect(page).to_not have_content "manual-trigger #2"
      end
    end
  end
end
