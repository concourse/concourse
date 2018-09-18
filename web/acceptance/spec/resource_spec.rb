describe 'resource', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before(:each) do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    dash_login

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
      within '.build-step .header' do
        expect(page).to have_content 'checking failed'
      end
      within '.step-body' do
        expect(page).to have_content 'failed: exit status'
      end
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
        expect(page.find('.build-metadata')).to have_content 'commit'
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
        expect(page.find('.build-metadata')).to have_content 'commit'

        fly("trigger-job -w -j other-pipeline/#{job_name}")
        visit dash_route("/teams/#{team_name}/pipelines/other-pipeline/resources/#{resource_name}")
        page.find('.list-collapsable-item', match: :first).click
        expect(page.find('.build-metadata')).to have_content 'commit'
      end
    end
  end

  describe 'navigating' do
    it 'can navigate to the resource' do
      visit dash_route("/teams/#{team_name}/pipelines/pipeline")

      page.find('a > text', text: 'some-resource').click

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

  describe 'last checked timestamp' do
    it 'shows last checked time' do
      fly('check-resource -r pipeline/some-resource')

      visit dash_route("/teams/#{team_name}/pipelines/pipeline")
      page.find('a > text', text: 'some-resource').click

      expect(page).to have_current_path("/teams/#{team_name}/pipelines/pipeline/resources/some-resource")
      expect(page).to have_css('h1', text: 'some-resource')
      expect(page).to have_css('.last-checked')
    end
  end

  describe 'pagination' do
    before do
      fly('set-pipeline -n -p pipeline -c fixtures/resource-checking.yml')
      fly('unpause-pipeline -p pipeline')
    end

    def with_timeout(timeout)
      Timeout.timeout timeout do
        begin
          yield
        rescue StandardError
          retry
        end
      end
    end

    it 'should have pagination for more than 100 resource versions' do
      with_timeout(60) { fly('check-resource -r pipeline/many-versions') }

      visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/many-versions")

      expect(page).to have_css('.resource-versions li', count: 100)

      expect(page).to have_css('.btn-page-link.prev.disabled')
      expect(page).to have_css('.btn-page-link.next')

      page.find('.btn-page-link.next').click

      expect(page).to have_css('.resource-versions li', count: 1)

      expect(page).to have_css('.btn-page-link.prev')
      expect(page).to have_css('.btn-page-link.next.disabled')

      page.find('.btn-page-link.prev').click

      expect(page).to have_css('.resource-versions li', count: 100)
    end

    it 'should not have pagination for less than 100 resource versions' do
      with_timeout(60) { fly('check-resource -r pipeline/few-versions') }

      visit dash_route("/teams/#{team_name}/pipelines/pipeline/resources/few-versions")

      expect(page).to have_css('.resource-versions li', count: 99)

      expect(page).to have_css('.btn-page-link.prev.disabled')
      expect(page).to have_css('.btn-page-link.next.disabled')
    end
  end
end
