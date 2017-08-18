require 'securerandom'
require 'color'

describe 'dashboard', type: :feature do
  let(:red) { Color::CSS['red'] }
  let(:green) { Color::CSS['green'] }
  let(:orange) { Color::CSS['orange'] }
  let(:yellow) { Color::CSS['yellow'] }
  let(:brown) { Color::CSS['brown'] }
  let(:blue) { Color::CSS['blue'] }
  let(:palette) { [red, green, orange, yellow, brown, blue] }

  let(:team_name) { generate_team_name }
  let(:other_team_name) { generate_team_name }

  before(:each) do
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')
    fly_with_input("set-team -n #{other_team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    fly('set-pipeline -n -p some-pipeline -c fixtures/test-pipeline.yml')
    fly('unpause-pipeline -p some-pipeline')

    fly_login other_team_name
    fly('set-pipeline -n -p other-pipeline-private -c fixtures/test-pipeline.yml')
    fly('unpause-pipeline -p other-pipeline-private')
    fly('set-pipeline -n -p other-pipeline-public -c fixtures/test-pipeline.yml')
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

  private

  def by_rgba(rgba)
    /rgba\((\d+),\s*(\d+),\s*(\d+), [^\)]+\)/.match(rgba) do |m|
      Color::RGB.new(m[1].to_i, m[2].to_i, m[3].to_i)
    end
  end
end
