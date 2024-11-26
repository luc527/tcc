defmodule Tccex.Message do
  @pub 1
  @sub 2
  @unsub 4

  def decode(<<@pub, topic::big-unsigned-integer-16, payload::binary>>) do
    {:ok, {:pub, topic, payload}}
  end

  def decode(<<@sub, topic::big-unsigned-integer-16>>) do
    {:ok, {:sub, topic}}
  end

  def decode(<<@unsub, topic::big-unsigned-integer-16>>) do
    {:ok, {:unsub, topic}}
  end

  def decode(b) when is_binary(b) and byte_size(b) == 0 do
    {:ok, :ping}
  end

  def decode(_), do: {:error, :invalid_type}

  def encode(:ping), do: << >>

  def encode({:sub, topic}),
    do: <<
      @sub::unsigned-integer-8,
      topic::big-unsigned-integer-16
    >>

  def encode({:unsub, topic}),
    do: <<
      @unsub::unsigned-integer-8,
      topic::big-unsigned-integer-16
    >>

  def encode({:pub, topic, payload}),
    do: <<
      @pub::unsigned-integer-8,
      topic::big-unsigned-integer-16,
      payload::binary
    >>
end
