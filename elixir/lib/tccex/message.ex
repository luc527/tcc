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

  def decode(<< >>) do
    {:ok, :ping}
  end

  def decode(_) do
    {:error, :invalid_type}
  end

  def encode(:ping) do
    <<>>
  end

  def encode({:sub, topic}) do
    <<
      @sub::unsigned-integer-8,
      topic::big-unsigned-integer-16
    >>
  end

  def encode({:unsub, topic}) do
    <<
      @unsub::unsigned-integer-8,
      topic::big-unsigned-integer-16
    >>
  end

  def encode({:pub, topic, payload}) do
    <<
      @pub::unsigned-integer-8,
      topic::big-unsigned-integer-16,
      payload::binary
    >>
  end
end
