require 'securerandom'
require 'color'

describe 'dashboard', type: :feature do
  let(:red) { Color::RGB.by_hex('E74C3C') }
  let(:green) { Color::RGB.by_hex('2ECC71') }
  let(:orange) { Color::RGB.by_hex('E67E22') }
  let(:yellow) { Color::RGB.by_hex('F1C40F') }
  let(:brown) { Color::RGB.by_hex('8F4B2D') }
  let(:blue) { Color::RGB.by_hex('3498DB') }
  let(:grey) { Color::RGB.by_hex('ECF0F1') }
  let(:palette) { [red, green, orange, yellow, brown, blue, grey] }

  let(:team_name) { generate_team_name }
  let(:other_team_name) { generate_team_name }

  before(:each) do
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')
    fly_with_input("set-team -n #{other_team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    fly('set-pipeline -n -p some-pipeline -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p some-pipeline')

    fly_login other_team_name
    fly('set-pipeline -n -p other-pipeline-private -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p other-pipeline-private')
    fly('set-pipeline -n -p other-pipeline-public -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p other-pipeline-public')
    fly('expose-pipeline -p other-pipeline-public')

    fly_login team_name
  end

  it 'shows all pipelins from the authenticated team and public pipelines from other teams' do
    dash_login team_name

    visit dash_route('/dashboard')

    within '.dashboard-team-group', text: team_name do
      expect(page).to have_content 'some-pipeline'
    end

    within '.dashboard-team-group', text: other_team_name do
      expect(page).to have_content 'other-pipeline-public'
      expect(page).to_not have_content 'other-pipeline-private'
    end
  end

  context 'when a pipeline is paused' do
    before do
      fly('pause-pipeline -p some-pipeline')
    end

    it 'is shown in blue' do
      dash_login team_name

      visit dash_route('/dashboard')

      pipeline = page.find('.dashboard-pipeline', text: 'some-pipeline')
      border_color = by_rgba(pipeline.native.css_value('border-top-color'))
      expect(border_color.closest_match(palette)).to eq(blue)
    end
  end

  context 'when a pipeline has a failed build' do
    before do
      fly_fail('trigger-job -w -j some-pipeline/failing')
    end

    it 'is shown in red' do
      dash_login team_name

      visit dash_route('/dashboard')

      pipeline = page.find('.dashboard-pipeline', text: 'some-pipeline')
      border_color = by_rgba(pipeline.native.css_value('border-top-color'))
      expect(border_color.closest_match(palette)).to eq(red)
    end
  end

  context 'when a pipeline has all passing builds' do
    before do
      fly('set-pipeline -n -p some-pipeline -c fixtures/passing-pipeline.yml')
      fly('trigger-job -w -j some-pipeline/passing')
    end

    it 'is shown in green' do
      dash_login team_name

      visit dash_route('/dashboard')

      pipeline = page.find('.dashboard-pipeline', text: 'some-pipeline')
      border_color = by_rgba(pipeline.native.css_value('border-top-color'))
      expect(border_color.closest_match(palette)).to eq(green)
    end
  end

  context 'when a pipeline has an aborted build' do
    before do
      fly('trigger-job -j some-pipeline/running')
      fly('abort-build -j some-pipeline/running -b 1')
    end

    it 'is shown in brown' do
      dash_login team_name

      visit dash_route('/dashboard')

      pipeline = page.find('.dashboard-pipeline', text: 'some-pipeline')
      border_color = by_rgba(pipeline.native.css_value('border-top-color'))
      expect(border_color.closest_match(palette)).to eq(brown)
    end
  end

  private

  def by_rgba(rgba)
    /rgba\((\d+),\s*(\d+),\s*(\d+), [^\)]+\)/.match(rgba) do |m|
      Color::RGB.new(m[1].to_i, m[2].to_i, m[3].to_i)
    end
  end
end
