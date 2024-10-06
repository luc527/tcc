defmodule Tcc.Utils do
  defmacro check(b, e) do
    quote do
      if(unquote(b), do: :ok, else: {:error, unquote(e)})
    end
  end

  def unique_integer() do
    :erlang.unique_integer([:positive, :monotonic])
  end
end
