require 'colors'

describe 'build', type: :feature do
  include Colors

  let(:team_name) { generate_team_name }

  before(:each) do
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    fly('set-pipeline -n -p pipeline -c fixtures/states-pipeline.yml')
    fly('unpause-pipeline -p pipeline')
    fly('trigger-job -j pipeline/running')

    dash_login team_name
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
