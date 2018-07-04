describe 'Nav', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }
  let(:pipeline_route) { "/teams/#{team_name}/pipelines/test-pipeline" }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    fly('set-pipeline -n -p test-pipeline -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p test-pipeline')

    dash_login
  end

  context 'on pipeline page' do
    before do
      visit dash_route(pipeline_route)
    end

    it 'includes the pipeline name' do
      within('.top-bar') do
        expect(page).to have_content 'test-pipeline'
      end
    end

    it 'the pipeline name is a link that resets the group' do
      expect(page).to have_link 'test-pipeline'

      other_group = page.find '.main', text: 'some-other-group'
      page.driver.browser.action.key_down(:shift)
          .click(other_group.native)
          .key_up(:shift).perform
      expect(page).to have_css '.active', text: 'some-group'
      expect(page).to have_css '.active', text: 'some-other-group'

      click_link 'test-pipeline'
      expect(page).to have_css '.active', text: 'some-group'
      expect(page).not_to have_css '.active', text: 'some-other-group'
    end

    it 'includes the group names' do
      within('.groups-bar') do
        expect(page).to have_content 'some-group'
      end
    end
  end

  context 'on resource page' do
    before do
      visit dash_route("/teams/#{team_name}/pipelines/test-pipeline/resources/some-resource")
    end

    it 'includes the pipeline name' do
      within('.top-bar') do
        expect(page).to have_content 'test-pipeline'
      end
    end

    it 'pipeline name links back to pipeline page' do
      within('.top-bar') do
        expect(page).to have_link 'test-pipeline', href: pipeline_route
      end
    end

    it 'includes the resource name' do
      within('.top-bar') do
        expect(page).to have_content 'some-resource'
      end
    end
  end

  context 'on job page' do
    before do
      visit dash_route("/teams/#{team_name}/pipelines/test-pipeline/jobs/resource-metadata")
    end

    it 'includes the pipeline name' do
      within('.top-bar') do
        expect(page).to have_content 'test-pipeline'
      end
    end

    it 'pipeline name links back to pipeline page' do
      within('.top-bar') do
        expect(page).to have_link 'test-pipeline', href: pipeline_route
      end
    end

    it 'includes the job name' do
      within('.top-bar') do
        expect(page).to have_content 'resource-metadata'
      end
    end
  end

  context 'on build page' do
    before do
      fly('trigger-job -w -j test-pipeline/resource-metadata')
      visit dash_route("/teams/#{team_name}/pipelines/test-pipeline/jobs/resource-metadata/builds/1")
    end

    it 'includes the pipeline name' do
      within('.top-bar') do
        expect(page).to have_content 'test-pipeline'
      end
    end

    it 'pipeline name links back to pipeline page' do
      within('.top-bar') do
        expect(page).to have_link 'test-pipeline', href: pipeline_route
      end
    end

    it 'includes the job name' do
      within('.top-bar') do
        expect(page).to have_content 'resource-metadata'
      end
    end
  end

  context 'pipeline name has special characters' do
    before do
      fly('set-pipeline -n -p "pipeline with special characters :)" -c fixtures/pipeline-with-special-characters.yml')
      fly('unpause-pipeline -p "pipeline with special characters :)"')
    end

    it 'renders special characters correctly' do
      visit dash_route("/teams/#{team_name}/pipelines/pipeline%20with%20special%20characters%20%3A%29")
      expect(page).to have_content 'pipeline with special characters :)'
      expect(page).not_to have_content '%20'

      visit dash_route("/teams/#{team_name}/pipelines/pipeline%20with%20special%20characters%20%3A%29/jobs/some-job")
      expect(page).to have_content 'pipeline with special characters :)'
      expect(page).to have_content 'some-job'

      visit dash_route("/teams/#{team_name}/pipelines/pipeline%20with%20special%20characters%20%3A%29/resources/some-resource")
      expect(page).to have_content 'pipeline with special characters :)'
      expect(page).to have_content 'some-resource'
    end

    it 'updates the title' do
      visit dash_route("/teams/#{team_name}/pipelines/pipeline%20with%20special%20characters%20%3A%29")
      expect(page).to have_title 'pipeline with special characters :) - Concourse'
    end
  end
end
