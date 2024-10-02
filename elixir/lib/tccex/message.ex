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

  @type outgoing ::
          :ping
          | {:jned, room :: integer, name :: binary}
          | {:hear, room :: integer, name :: binary, text :: binary}
          | {:exed, room :: integer, name :: binary}
          | {:rols, room_list_csv :: binary}
          | {:mprob, error_code :: integer}

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

  @spec encode(outgoing) :: binary

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
    do: encode({:prob, transient_code(type_code!(type))})

  def encode({:prob, code}) when is_atom(code),
    do: encode({:prob, prob_code!(code)})

  def encode({:prob, code}) when is_integer(code),
    do: <<@mprob, code::little-integer-32>>

  def encode({:rols, rooms}) do
    lines = Enum.map(rooms, fn {room, name} -> "#{room},#{name}\n" end)
    lines = ["room,name\n" | lines]
    csv = Enum.join(lines)
    <<@mrols, byte_size(csv)::little-integer-16, csv::binary>>
  end

  def is_error_for(error, given_type) do
    with {:ok, prob_code} <- prob_code(error),
         {:ok, given_type_code} <- type_code(given_type) do
      prob_type_code = prob_code |> Bitwise.bsr(8) |> Bitwise.band(0xFF)
      Bitwise.band(given_type_code, prob_type_code) > 0
    else
      _ -> false
    end
  end

  defp prob_code(:bad_type), do: {:ok, @ebadtype}
  defp prob_code(:bad_name), do: {:ok, @ebadname}
  defp prob_code(:joined), do: {:ok, @ejoined}
  defp prob_code(:name_in_use), do: {:ok, @enameinuse}
  defp prob_code(:room_limit), do: {:ok, @eroomlimit}
  defp prob_code(:room_full), do: {:ok, @eroomfull}
  defp prob_code(:bad_message), do: {:ok, @ebadmes}
  defp prob_code(:bad_room), do: {:ok, @ebadroom}
  defp prob_code(_), do: :error

  defp prob_code!(error) do
    {:ok, code} = prob_code(error)
    code
  end

  defp type_code(:pong), do: {:ok, @mpong}
  defp type_code(:join), do: {:ok, @mjoin}
  defp type_code(:exit), do: {:ok, @mexit}
  defp type_code(:talk), do: {:ok, @mtalk}
  defp type_code(:lsro), do: {:ok, @mlsro}
  defp type_code(_), do: :error

  defp type_code!(type) do
    {:ok, code} = type_code(type)
    code
  end

  defp transient_code(code) do
    code |> Bitwise.bsl(8) |> Bitwise.bor(@etransientsuffix)
  end
end
