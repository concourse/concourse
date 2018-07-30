require 'colors'

describe 'pipeline', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before(:each) do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    fly('set-pipeline -n -p test-pipeline -c fixtures/simple-pipeline.yml')
    fly('set-pipeline -n -p other-pipeline -c fixtures/simple-pipeline.yml')

    dash_login
    visit dash_route("/teams/#{team_name}/pipelines/test-pipeline")
  end

  context 'with a failing output resource' do
    include Colors

    let(:node) { page.find('.node.output', text: 'broken-time') }
    let(:rect) { node.find('rect') }

    before do
      fly('set-pipeline -n -p states-pipeline -c fixtures/states-pipeline.yml')
      fly('unpause-pipeline -p states-pipeline')
      fly_fail('check-resource -r states-pipeline/broken-time')
      visit dash_route("/teams/#{team_name}/pipelines/states-pipeline")
    end

    it 'shows the resource node in amber' do
      expect(fill_color(rect)).to eq AMBER
    end

    it 'when the resource is paused shows the node in blue' do
      node.click
      page.find('.btn-pause').click
      visit dash_route("/teams/#{team_name}/pipelines/states-pipeline")
      expect(fill_color(rect)).to eq BLUE
    end
  end
end
