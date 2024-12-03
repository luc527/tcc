defmodule Tccex.Rot13sort do
  def of(s) when is_binary(s), do: of(String.to_charlist(s))
  def of(l) when is_list(l), do: of(l, [])

  # XXX: maybe the string conversion is unecessary, since all Erlang io things understand charlists
  # and even iolists

  defp of([],      acc), do: acc |> Enum.sort |> List.to_string
  defp of([h | t], acc), do: of(t, rot(h, acc))

  defp rot(c, l) when c in ?a..?z, do: [rem(c - ?a + 13, 26) + ?a | l]
  defp rot(c, l) when c in ?A..?Z, do: [rem(c - ?A + 13, 26) + ?A | l]
  defp rot(_, l), do: l
end
