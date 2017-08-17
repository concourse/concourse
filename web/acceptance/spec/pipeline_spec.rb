require 'greenletters'

describe 'test pilot', type: :feature do
  def fly(command)
    `fly -t testpilot #{command}`
  end

  def atc_route(path='')
    URI.join ENV.fetch('ATC_URL', 'http://127.0.0.1:8080'), path
  end

  before(:all) do
    fly("login -c #{atc_route} -n main")
    fly('sp -n -p test-pipeline -c fixtures/test-pipeline.yml')
    fly('ep -p test-pipeline')
  end

  before(:each) do
    visit atc_route('/teams/main/pipelines/test-pipeline')
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
