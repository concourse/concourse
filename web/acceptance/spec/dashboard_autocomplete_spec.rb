describe 'dashboard autocomplete', type: :feature do
  let(:team_name) { generate_team_name }
  let(:fly_home) { Dir.mktmpdir }

  before do
    fly_login 'main'
    fly_with_input("set-team -n #{team_name} --local-user=#{ATC_USERNAME}", 'y')

    fly_login team_name
    dash_login

    fly('set-pipeline -n -p some-pipeline -c fixtures/dashboard-pipeline.yml')
    fly('unpause-pipeline -p some-pipeline')

    fly('set-pipeline -n -p other-pipeline -c fixtures/dashboard-pipeline.yml')
    fly('unpause-pipeline -p other-pipeline')

    visit dash_route
  end

  context 'without focus' do
    it 'does not display any options' do
      expect(page).to have_no_content 'status:'
      expect(page).to have_no_content 'team:'
    end
  end

  context 'with focus' do
    context 'with empty query' do
      it 'shows the default options' do
        find_field('search-input-field').click
        expect(page).to have_content 'status:'
        expect(page).to have_content 'team:'
      end
    end

    context 'with a status query' do
      it 'shows the matching options' do
        search 'status:'
        expect(page).to have_content 'status: paused'
        expect(page).to have_content 'status: pending'
        expect(page).to have_content 'status: failed'
        expect(page).to have_content 'status: errored'
        expect(page).to have_content 'status: aborted'
        expect(page).to have_content 'status: running'
        expect(page).to have_content 'status: succeeded'
      end

      it 'populates the search box when clicking on an option' do
        search 'status:'
        find('li', text: 'status: paused').click
        expect(page).to have_no_content 'status: failed'
        expect(page).to have_field('search-input-field', with: 'status: paused')
      end

      it 'supports arrow keys' do
        search 'status:'
        find_field('search-input-field').native.send_keys :down
        find_field('search-input-field').native.send_keys :down
        find_field('search-input-field').native.send_keys :down
        find_field('search-input-field').native.send_keys :down
        find_field('search-input-field').native.send_keys :up
        find_field('search-input-field').native.send_keys :enter
        expect(page).to have_field 'search-input-field', with: 'status: failed'
        expect(page).to have_text 'No results'
      end

      it 'blurs when escape key is pressed' do
        find_field('search-input-field').click
        find_field('search-input-field').native.send_keys :escape
        expect(page).to have_no_content 'status: '
        expect(page).to have_no_content 'team: '
      end
    end

    context 'with a team query' do
      it 'shows the matching options' do
        search 'team:'
        expect(page).to have_content 'team: main'
        expect(page).to have_content "team: #{team_name}"
      end

      context 'with many teams' do
        let(:teams) { (1..15).map { generate_team_name } }

        before do
          fly_login 'main'
          teams.each do |team|
            fly_with_input("set-team -n #{team} --local-user=#{ATC_USERNAME}", 'y')
          end
        end

        it 'shows a max of 10 teams' do
          visit dash_route
          search 'team:'
          expect(page).to have_css '.search-option', count: 10
        end

        after do
          fly_login 'main'
          teams.each do |team|
            fly_with_input("destroy-team -n #{team}", team)
          end
        end
      end
    end
  end

  private

  def search(term)
    term.split('').each { |c| find_field('search-input-field').native.send_keys(c) }
  end
end
