defmodule Tccex.Sumall do
  def of(s) when is_binary(s), do: of(String.to_charlist(s))
  def of(l) when is_list(l) do
    Enum.map(l, fn c0 ->
      c =
        Stream.flat_map(0..length(l)-1, fn j ->
          Enum.map(l, fn c1 -> c0 * j + c1 end)
        end)
        |> Enum.reduce(0, fn a, b -> Bitwise.band(a + b, 0xFF) end)
      rem(c, 26) + ?a
    end)
    |> List.to_string
  end

end
