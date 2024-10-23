defmodule Tccex.Message do
  @ping 0x01
  @pub 0x02
  @sub 0x03
  @unsub 0x04
  @subbed 0x05

  def decode(binary) do
    decode(binary, [])
  end

  def decode(<<@ping, rest::binary>>, messages) do
    decode(rest, [:ping | messages])
  end

  def decode(
        <<
          @pub::unsigned-integer-8,
          topic::unsigned-little-integer-16,
          len::unsigned-little-integer-16,
          payload::binary-size(len),
          rest::binary
        >>,
        messages
      ) do
    decode(rest, [{:pub, topic, payload} | messages])
  end

  def decode(
        <<
          @sub::unsigned-integer-8,
          topic::unsigned-little-integer-16,
          rest::binary
        >>,
        messages
      ) do
    decode(rest, [{:sub, topic} | messages])
  end

  def decode(
        <<
          @unsub::unsigned-integer-8,
          topic::unsigned-little-integer-16,
          rest::binary
        >>,
        messages
      ) do
    decode(rest, [{:sub, topic} | messages])
  end

  def decode(<<type::unsigned-integer-8, rest::binary>>, messages)
      when type != @ping and type != @pub and type != @sub do
    {:error, {:unknown_type, type}, Enum.reverse(messages), rest}
  end

  def decode(<<rest::binary>>, messages) do
    {:ok, Enum.reverse(messages), rest}
  end

  def encode(:ping) do
    <<@ping::unsigned-integer-8>>
  end

  def encode({:pub, topic, payload}) do
    <<
      @pub::unsigned-integer-8,
      topic::unsigned-little-integer-16,
      byte_size(payload)::unsigned-little-integer-16,
      payload::binary
    >>
  end

  def encode({:subbed, topic, true}) do
    <<@subbed::unsigned-integer-8, topic::unsigned-little-integer-16, 1::unsigned-integer-8>>
  end

  def encode({:subbed, topic, false}) do
    <<@subbed::unsigned-integer-8, topic::unsigned-little-integer-16, 0::unsigned-integer-8>>
  end
end
