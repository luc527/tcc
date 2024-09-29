defmodule Tccex.Tcp.Receiver do
  use Task
  require Logger

  def start_link({sock, client_pid}) do
    Task.start_link(fn -> loop(sock, client_pid) end)
  end

  defp loop(sock, client_pid) do
    loop(sock, client_pid, <<>>)
  end

  defp loop(sock, client_pid, prev_rest) do
    case :gen_tcp.recv(sock, 0) do
      {:error, :closed} ->
        Logger.info("closed")
        :ok

      {:ok, packet} ->
        rest = handle_packet(sock, client_pid, prev_rest <> packet)
        loop(sock, client_pid, rest)
    end
  end

  defp error_to_prob(:invalid_message_type), do: {:prob, :bad_type}

  defp handle_packet(sock, client_pid, packet) do
    {messages, errors, rest} = Tccex.Message.decode_all(packet)
    Enum.each(messages, &recv_message(client_pid, &1))
    Enum.each(errors, &send_error(sock, &1))
    rest
  end

  defp recv_message(client_pid, message) do
    Tccex.Client.recv(client_pid, message)
  end

  defp send_error(sock, error) do
    packet = error |> error_to_prob |> Tccex.Message.encode
    :gen_tcp.send(sock, packet)
  end
end
