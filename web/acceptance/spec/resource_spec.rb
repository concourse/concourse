describe 'resource', type: :feature do
  let(:team_name) { generate_team_name }

  before(:each) do
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    dash_login team_name
  end

  describe 'broken resource' do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/states-pipeline.yml')
      fly('unpause-pipeline -p pipeline')
      fly_fail('check-resource -r pipeline/broken-time')
    end

    it 'displays logs correctly' do
      resource_name = 'broken-time'
      visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/#{resource_name}")
      expect(page).to have_content 'failed: exit status'
    end
  end

  describe 'resource metadata' do
    context 'when running build again on the same job' do
      before do
        fly('set-pipeline -n -p pipeline -c fixtures/states-pipeline.yml')
        fly('unpause-pipeline -p pipeline')
      end

      it 'prints resource metadata' do
        job_name = 'resource-metadata'
        fly("trigger-job -w -j pipeline/#{job_name}")
        fly("trigger-job -w -j pipeline/#{job_name}")
        resource_name = 'some-resource'
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/#{resource_name}")
        page.find('.list-collapsable-item', match: :first).click
        expect(page.find('.build-metadata')).to have_content 'image'
      end
    end

    context 'when running build on a another pipeline with the same resource config' do
      before do
        fly('set-pipeline -n -p pipeline -c fixtures/states-pipeline.yml')
        fly('unpause-pipeline -p pipeline')
        fly('set-pipeline -n -p other-pipeline -c fixtures/states-pipeline.yml')
        fly('unpause-pipeline -p other-pipeline')
      end

      it 'prints resource metadata' do
        job_name = 'resource-metadata'
        resource_name = 'some-resource'

        fly("trigger-job -w -j pipeline/#{job_name}")
        visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/#{resource_name}")
        page.find('.list-collapsable-item', match: :first).click
        expect(page.find('.build-metadata')).to have_content 'image'

        fly("trigger-job -w -j other-pipeline/#{job_name}")
        visit dash_route("/teams/#{team_name}/pipelines/other-pipeline/resources/#{resource_name}")
        page.find('.list-collapsable-item', match: :first).click
        expect(page.find('.build-metadata')).to have_content 'image'
      end
    end
  end
end
