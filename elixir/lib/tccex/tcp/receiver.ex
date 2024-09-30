defmodule Tccex.Tcp.Receiver do
  use Task
  alias Tccex.{Client, Message}
  require Logger

  @recvTimeout 30_000

  def start_link({sock, client_id}) do
    Task.start_link(fn -> run(sock, client_id) end)
  end

  defp run(sock, client_id) do
    try do
      :ok = loop(sock, client_id)
    after
      Logger.info "closed, client_id: #{client_id}"
      :gen_tcp.close(sock)
    end
  end

  defp loop(sock, client_id) do
    loop(sock, client_id, <<>>)
  end

  defp loop(sock, client_id, prev_rest) do
    with {:ok, packet} <- :gen_tcp.recv(sock, 0, @recvTimeout) do
      rest = handle_packet(sock, client_id, prev_rest <> packet)
      loop(sock, client_id, rest)
    else
      {:error, :timeout} ->
        :gen_tcp.shutdown(sock, :write)
        loop(sock, client_id, prev_rest)

      {:error, :closed} ->
        :ok
    end
  end

  defp error_to_prob(:invalid_message_type), do: {:prob, :bad_type}

  defp handle_packet(sock, client_id, packet) do
    {messages, errors, rest} = Message.decode_all(packet)
    Enum.each(messages, &recv_message(client_id, &1))
    Enum.each(errors, &send_error(sock, &1))
    rest
  end

  defp recv_message(client_id, message) do
    Client.recv(client_id, message)
  end

  defp send_error(sock, error) do
    packet = error |> error_to_prob |> Message.encode()
    :gen_tcp.send(sock, packet)
  end
end
