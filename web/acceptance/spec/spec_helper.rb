require 'capybara/rspec'
require 'selenium/webdriver'
require 'stringio'
require 'fly'
require 'dash'

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

Capybara.register_driver :chrome do |app|
  Capybara::Selenium::Driver.new(app, browser: :chrome)
end

Capybara.register_driver :headless_chrome do |app|
  capabilities = Selenium::WebDriver::Remote::Capabilities.chrome(
    chromeOptions: { args: %w[headless disable-gpu no-sandbox] }
  )

  Capybara::Selenium::Driver.new app,
                                 browser: :chrome,
                                 desired_capabilities: capabilities
end

Capybara.default_driver = :headless_chrome
Capybara.javascript_driver = :headless_chrome
