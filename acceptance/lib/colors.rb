require 'rspec/expectations'
require 'color'

RED = Color::CSS['red']
GREEN = Color::CSS['green']
ORANGE = Color::CSS['orange']
YELLOW = Color::CSS['yellow']
BROWN = Color::CSS['brown']
BLUE = Color::CSS['blue']
PALETTE = [RED, GREEN, ORANGE, YELLOW, BROWN, BLUE].freeze

RSpec::Matchers.define :be_greyscale do |expected|
  match do |actual|
    actual.to_grayscale.to_rgb.closest_match(PALETTE + [actual], :jnd) == expected
  end

  diffable
end

module Colors
  def background_color(element)
    by_rgb(computed_style(element, 'backgroundColor')).closest_match(PALETTE)
  end

  def background_palette(element)
    background_color(element).closest_match(PALETTE)
  end

  def computed_style(node, attribute)
    page.evaluate_script("window.getComputedStyle(document.evaluate('#{node.path}', document, null, XPathResult.FIRST_ORDERED_NODE_TYPE, null).singleNodeValue).#{attribute}")
  end

  def by_rgb(rgb)
    /rgb\((\d+),\s*(\d+),\s*(\d+)\)/.match(rgb) do |m|
      Color::RGB.new(m[1].to_i, m[2].to_i, m[3].to_i)
    end
  end
end
