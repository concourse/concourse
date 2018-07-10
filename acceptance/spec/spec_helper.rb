require 'capybara/rspec'
require 'selenium/webdriver'
require 'stringio'
require 'fly'
require 'dash'
require 'tmpdir'

ATC_URL = ENV.fetch('ATC_URL', 'http://localhost:8080').freeze

ATC_USERNAME = ENV.fetch('ATC_USERNAME', "test#{ENV['TEST_ENV_NUMBER']}").freeze
ATC_PASSWORD = ENV.fetch('ATC_PASSWORD', "test#{ENV['TEST_ENV_NUMBER']}").freeze

RSpec.configure do |config|
  include Fly
  config.include Dash

  config.before(:suite) do
    def team_name
      'wats' # fly target
    end

    def fly_home
      @fly_home ||= Dir.mktmpdir
    end

    fly_login('main')
    fly('teams').each_line do |team|
      next unless team.start_with? 'wats-team'
      fly("destroy-team -n #{team.chomp} --non-interactive")
    end
  end

  config.after(:each) do
    tries = 3
    begin
      fly_login 'main'
      fly_with_input("destroy-team -n #{team_name}", team_name)
      fly('logout')
    rescue StandardError
      # there is a chance for deadlock deleting the team while
      # resource checking is saving new versions
      sleep 60
      tries -= 1
      retry if tries > 0
    end
  end
end

Capybara.register_driver :chrome do |app|
  Capybara::Selenium::Driver.new(app, browser: :chrome)
end

Capybara.register_driver :headless_chrome do |app|
  options = Selenium::WebDriver::Chrome::Options.new(
    args: %w[
      headless
      disable-gpu
      no-sandbox
      window-size=2560,1440
    ]
  )

  Capybara::Selenium::Driver.new app, browser: :chrome, options: options
end

Capybara.default_driver = :headless_chrome
Capybara.javascript_driver = :headless_chrome

Capybara.save_path = '/tmp'

Capybara.default_max_wait_time = ATC_URL.include?('localhost') ? 10 : 60
