describe 'resource', type: :feature do
  let(:team_name) { generate_team_name }

  before(:each) do
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    dash_login team_name

    fly('set-pipeline -n -p pipeline -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p pipeline')
  end

  describe 'broken resource' do
    before do
      fly_fail('check-resource -r pipeline/broken-time')
    end

    it 'displays logs correctly' do
      resource_name = 'broken-time'
      visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/#{resource_name}")
      expect(page).to have_content('checking failed')
      expect(page).to have_content 'failed: exit status'
    end
  end

  describe 'resource metadata' do
    context 'when running build again on the same job' do
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

  describe 'navigating' do
    it 'can navigate to the resource' do
      visit dash_route

      page.find('a', text: 'some-resource').click

      expect(page).to have_current_path("/teams/#{team_name}/pipelines/pipeline/resources/some-resource")
      expect(page).to have_css('h1', text: 'some-resource')
    end
  end

  describe 'pausing' do
    it 'can pause an unpaused resource' do
      fly('unpause-resource -r pipeline/some-resource')

      visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/some-resource")

      expect(page).to have_css('.btn-pause.disabled')

      page.find('.btn-pause').click

      expect(page).to have_css('.btn-pause.enabled')
    end

    it 'can unpause a paused resource' do
      fly('pause-resource -r pipeline/some-resource')

      visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/some-resource")

      expect(page).to have_css('.btn-pause.enabled')

      page.find('.btn-pause').click

      expect(page).to have_css('.btn-pause.disabled')
    end
  end

  describe 'resource pagination' do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/resource-checking.yml')
      fly('unpause-pipeline -p pipeline')
    end

    it 'shows pagination for more than 100 resource versions' do
      resource_name = 'warp-time'
      visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/#{resource_name}")

      button = page.find(".btn-page-link.next.disabled")
      expect(button).to_not be_nil
      expect(page.all('.resource-versions li').count).to be < 100
      counter = page.all('.resource-versions li').count
      while counter < 100 do
        fly('check-resource -r pipeline/warp-time')
        counter = page.all('.resource-versions li').count
      end
      page.find(".btn-page-link.next").click
      expect(page.all('.resource-versions li').count).to be >= 0
    end
  end
end
