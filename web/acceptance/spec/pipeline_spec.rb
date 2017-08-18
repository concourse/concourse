describe 'pipeline', type: :feature do
  before(:all) do
    fly_login 'main'
    fly('set-pipeline -n -p test-pipeline -c fixtures/test-pipeline.yml')
    fly('expose-pipeline -p test-pipeline')
  end

  before(:each) do
    visit dash_route('/teams/main/pipelines/test-pipeline')
  end

  it 'displays the unescaped names in the pipeline view' do
    expect(page.find('.job')).to have_content 'some/job'
    expect(page.find('.input')).to have_content 'some/resource'
  end

  it 'can navigate to the escaped links' do
    page.find('a', text: 'some/job').click
    expect(page).to have_current_path '/teams/main/pipelines/test-pipeline/jobs/some%2Fjob'

    page.go_back

    page.find('a', text: 'some/resource').click
    expect(page).to have_current_path '/teams/main/pipelines/test-pipeline/resources/some%2Fresource'
  end
end
