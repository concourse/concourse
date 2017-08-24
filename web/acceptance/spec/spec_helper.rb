require 'capybara/rspec'
require 'selenium/webdriver'
require 'capybara/poltergeist'
require 'stringio'
require 'fly'
require 'dash'

Capybara.default_driver = :poltergeist
Capybara.javascript_driver = :poltergeist

ATC_URL = ENV.fetch('ATC_URL', 'http://127.0.0.1:8080').freeze

RSpec.configure do |config|
  include Fly
  config.include Dash

  config.before(:suite) do
    fly_login 'main'
    cleanup_teams
  end

  config.before(:each) do
    fly_login 'main'
  end

  config.after(:suite) do
    fly_login 'main'
    cleanup_teams
  end
end
