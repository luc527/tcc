defmodule Tccex.Message do
  @mping 0x80
  @mpong 0x00
  @mtalk 0x01
  @mhear 0x81
  @mjoin 0x02
  @mjned 0x82
  @mexit 0x04
  @mexed 0x84
  @mlsro 0x08
  @mrols 0x88
  @mprob 0x90

  @ebadtype @mprob |> Bitwise.bnot() |> Bitwise.bsl(8)

  @ejoined @mjoin |> Bitwise.bsl(8) |> Bitwise.bor(0x01)
  @ebadname @mjoin |> Bitwise.bsl(8) |> Bitwise.bor(0x02)
  @enameinuse @mjoin |> Bitwise.bsl(8) |> Bitwise.bor(0x03)
  @eroomlimit @mjoin |> Bitwise.bsl(8) |> Bitwise.bor(0x04)
  @eroomfull @mjoin |> Bitwise.bsl(8) |> Bitwise.bor(0x05)

  @ebadmes @mtalk |> Bitwise.bsl(8) |> Bitwise.bor(0x01)

  @ebadroom Bitwise.bor(@mtalk, @mexit) |> Bitwise.bsl(8) |> Bitwise.bor(0x01)

  @etransientsuffix 0xFF

  @type incoming ::
          :pong
          | {:join, room :: integer, name :: binary}
          | {:exit, room :: integer}
          | {:talk, room :: integer, text :: binary}
          | :lsro

  @type decode_error :: :invalid_message_type

  defp decode_step(_)

  defp decode_step(<<>>), do: {:partial, <<>>}

  defp decode_step(<<@mpong, rest::binary>>), do: {:ok, :pong, rest}

  defp decode_step(
         <<@mjoin, room::little-integer-32, namelen::little-integer-8, name::binary-size(namelen),
           rest::binary>>
       ),
       do: {:ok, {:join, room, name}, rest}

  defp decode_step(<<@mjoin, _::binary>> = b), do: {:partial, b}

  defp decode_step(<<@mexit, room::little-integer-32, rest::binary>>),
    do: {:ok, {:exit, room}, rest}

  defp decode_step(<<@mexit, _::binary>> = b), do: {:partial, b}

  defp decode_step(
         <<@mtalk, room::little-integer-32, textlen::little-integer-16,
           text::binary-size(textlen), rest::binary>>
       ),
       do: {:ok, {:talk, room, text}, rest}

  defp decode_step(<<@mtalk, _::binary>> = b), do: {:partial, b}

  defp decode_step(<<@mlsro, rest::binary>>), do: {:ok, :lsro, rest}

  defp decode_step(<<_, rest::binary>>),
    do: {:error, :invalid_message_type, rest}

  @spec decode_all(binary) :: {messages :: [incoming], errors :: [term], rest :: binary}

  def decode_all(binary),
    do: decode_all(binary, [], [])

  defp decode_all(binary, errors, messages) do
    case decode_step(binary) do
      {:ok, message, rest} ->
        decode_all(rest, errors, [message | messages])

      {:error, error, rest} ->
        decode_all(rest, [error | errors], messages)

      {:partial, rest} ->
        {Enum.reverse(messages), Enum.reverse(errors), rest}
    end
  end

  def encode(:ping), do: <<@mping>>

  def encode({:jned, room, name}),
    do: <<@mjned, room::little-integer-32, byte_size(name)::little-integer-8, name::binary>>

  def encode({:exed, room, name}),
    do: <<@mexed, room::little-integer-32, byte_size(name)::little-integer-8, name::binary>>

  def encode({:hear, room, name, text}),
    do:
      <<@mhear, room::little-integer-32, byte_size(name)::little-integer-8,
        byte_size(text)::little-integer-16, name::binary, text::binary>>

  def encode({:prob, {:transient, type}}) when is_atom(type),
    do: encode({:prob, transient_code(type_code(type))})

  def encode({:prob, code}) when is_atom(code),
    do: encode({:prob, prob_code(code)})

  def encode({:prob, code}) when is_integer(code),
    do: <<@mprob, code::little-integer-32>>

  def encode({:rols, rooms}) do
    lines =
      rooms
      |> Enum.map(fn {room, name} -> "#{room},#{name}\n" end)
      |> Enum.join()

    csv = "room,name\n" <> lines
    <<@mrols, byte_size(csv)::little-integer-16, csv::binary>>
  end

  defp prob_code(:bad_type), do: @ebadtype
  defp prob_code(:bad_name), do: @ebadname
  defp prob_code(:joined), do: @ejoined
  defp prob_code(:name_in_use), do: @enameinuse
  defp prob_code(:room_limit), do: @eroomlimit
  defp prob_code(:room_full), do: @eroomfull
  defp prob_code(:bad_message), do: @ebadmes
  defp prob_code(:bad_room), do: @ebadroom

  defp type_code(:pong), do: @mpong
  defp type_code(:join), do: @mjoin
  defp type_code(:exit), do: @mexit
  defp type_code(:talk), do: @mtalk
  defp type_code(:lsro), do: @mlsro

  defp transient_code(code), do: code |> Bitwise.bsl(8) |> Bitwise.bor(@etransientsuffix)
end
