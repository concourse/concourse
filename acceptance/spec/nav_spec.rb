describe 'Nav', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }
  let(:pipeline_route) { "/teams/#{team_name}/pipelines/test-pipeline" }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    fly('set-pipeline -n -p test-pipeline -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p test-pipeline')

    dash_login team_name
  end

  context 'on home page' do
    before do
      visit dash_route("/")
      expect(page).to have_content 'resource-metadata'
    end

    it 'includes the pipeline name' do
      within('.top-bar') do
        expect(page).to have_content 'test-pipeline'
      end
    end

    it 'the pipeline name is not a link' do
      expect(page).not_to have_link 'test-pipeline'
    end

    it 'includes the group names' do
      within('.groups-bar') do
        expect(page).to have_content 'some-group'
      end
    end
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

    it 'the pipeline name is not a link' do
      expect(page).not_to have_link 'test-pipeline'
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
end
