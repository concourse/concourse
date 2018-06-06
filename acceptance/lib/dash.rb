module Dash
  def dash_route(path = '')
    URI.join ATC_URL, path
  end

  def dash_login
    visit dash_route('/sky/login?redirect_uri=/')
    fill_in 'password', with: ATC_USERNAME
    fill_in 'login', with: ATC_PASSWORD
    click_button 'login'
    expect(page).to_not have_content 'login'
  end
end
