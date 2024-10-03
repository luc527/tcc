defmodule Tccex.Utils do
  defmacro check(b, e) do
    quote do
      if(unquote(b), do: :ok, else: {:error, unquote(e)})
    end
  end
end
