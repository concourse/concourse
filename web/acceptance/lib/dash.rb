module Dash
  def dash_route(path = '')
    URI.join ATC_URL, path
  end

  def dash_login(team_name)
    visit ATC_URL
    click_on 'login'
    click_on team_name
    click_button 'login'

    # wait for logging in to complete, since the following 'visit'
    # will not wait, potentially interrupting the JS and preventing
    # the session from being saved
    expect(page).to_not have_content 'login'
  end
end
