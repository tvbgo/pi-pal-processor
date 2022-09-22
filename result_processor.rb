require 'json'
Dir.glob("full_results/batch-*.txt") do |file|

  f = File.open(file)

  f.each do |line|
    r = line.split(',')
    last_digit = r[2][-1]
    size = r[3].to_i
    pal = r[2].strip

    if size.to_i > 19 && !['0', '2', '4', '6', '8'].include?(last_digit)
      p "Validating position..."
      position = (r[0].to_i + r[1].to_i) - ((size - 1) / 2)
      test_call = `curl https://api.pi.delivery/v1/pi?start=#{position + 1}&numberOfDigits=#{size}&radix=10`
      res = JSON.parse(test_call)
      p res["content"][0..(size - 1)] == pal
      p "curl https://api.pi.delivery/v1/pi?start=#{position + 1}&numberOfDigits=#{size}&radix=10"
    end
  end
end
