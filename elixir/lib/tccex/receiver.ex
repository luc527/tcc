defmodule Tccex.Receiver do
  alias Tccex.Client
  alias Tccex.Message
  require Logger
  use Task

  @recv_timeout_ms 60_000

  def start_link({sock, client_pid}) do
    Task.start_link(fn -> run(sock, client_pid) end)
  end

  defp run(sock, client_pid) do
    recv_loop(sock, client_pid, <<>>)
  end

  defp recv_loop(sock, client_pid, prev_rest) do
    with {:ok, packet} <- :gen_tcp.recv(sock, 0, @recv_timeout_ms),
         {:ok, messages, rest} <- Message.decode(prev_rest <> packet) do
      Enum.each(messages, &handle_message(&1, client_pid))
      recv_loop(sock, client_pid, rest)
    else
      error ->
        :gen_tcp.shutdown(sock, :write)

        case error do
          {:error, {:unknown_type, type}, _messages, _rest} ->
            Logger.warning("receiver stopping because of weird message, type: #{inspect(type)}")

          {:error, reason} ->
            Logger.info("receiver stopping with reason #{inspect(reason)}")
        end
    end
  end

  defp handle_message(:ping, client_pid), do: Client.ping(client_pid)
  defp handle_message({:sub, topic}, client_pid), do: Client.sub(client_pid, topic)
  defp handle_message({:unsub, topic}, client_pid), do: Client.unsub(client_pid, topic)

  defp handle_message({:pub, topic, payload}, client_pid),
    do: Client.pub(client_pid, topic, payload)
end
