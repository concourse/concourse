module Dash
  def dash_route(path = '')
    URI.join ATC_URL, path
  end

  def dash_login(team_name)
    visit dash_route("/teams/#{team_name}/login")
    click_button 'login'
    expect(page).to_not have_content 'login'
  end
end
