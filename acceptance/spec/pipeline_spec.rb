describe 'pipeline', type: :feature do
  let(:team_name) { generate_team_name }

  before(:each) do
    fly_with_input("set-team -n #{team_name} --no-really-i-dont-want-any-auth", 'y')

    fly_login team_name
    fly('set-pipeline -n -p test-pipeline -c fixtures/test-pipeline.yml')
    fly('expose-pipeline -p test-pipeline')
    visit dash_route("/teams/#{team_name}/pipelines/test-pipeline")
  end

  it 'displays the unescaped names in the pipeline view' do
    expect(page.find('.job')).to have_content 'some/job'
    expect(page.find('.input')).to have_content 'some/resource'
  end

  it 'can navigate to the escaped links' do
    page.find('a', text: 'some/job').click
    expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline/jobs/some%2Fjob"

    page.go_back

    page.find('a', text: 'some/resource').click
    expect(page).to have_current_path "/teams/#{team_name}/pipelines/test-pipeline/resources/some%2Fresource"
  end
end
