defmodule Tcc.MockClient do
  require Logger
  def start(id) do
    spawn(fn ->
      {:ok, ^id} = Tcc.Clients.register(id)
      loop(id)
    end)
  end

  defp log_on_error(:ok, _id), do: :ok
  defp log_on_error(error, id), do: Logger.info("<#{id}> ! #{inspect(error)}")

  def loop(id) do
    receive do
      :quit ->
        :ok

      {:message, message} ->
        Logger.info("<#{id}> #{inspect(message)}")
        loop(id)

      {:join, room, name} ->
        Tcc.Clients.join(room, name, id) |> log_on_error(id)
        loop(id)

      {:exit, room} ->
        Tcc.Clients.exit(room, id) |> log_on_error(id)
        loop(id)

      {:talk, room, text} ->
        Tcc.Clients.talk(room, text, id) |> log_on_error(id)
        loop(id)

        # TODO: lsro
    end
  end
end
