require 'json'

cand_array = []
Dir.glob("full_results/*.txt") do |file|
  f = File.open(file)

  f.each do |line|
    candidate = { size: 0, position: 0, pal: 0 }
    r = line.split(',')
    last_digit = r[2][-1]
    size = r[3].to_i
    pal = r[2].strip

    if size.to_i >= 25 && !['0', '2', '4', '5', '6', '8'].include?(last_digit)
      position = (r[0].to_i + r[1].to_i) - ((size - 1) / 2) + 1
      candidate[:size] = size
      candidate[:position] = position
      candidate[:pal] = pal
      test_call = `curl https://api.pi.delivery/v1/pi?start=#{position}&numberOfDigits=#{size}&radix=10`
      res = JSON.parse(test_call)
      candidate[:position_validated] = res["content"][0..(size - 1)] == pal
      cand_array << candidate
    end
  end
end

cand_array.sort_by { |i| i[:size] }.reverse.each { |i| p i }
