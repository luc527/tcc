defmodule Tcc.Tcp.Receiver do
  require Logger
  alias Tcc.{Message, Client}

  @recv_timeout 30_000

  def receive_loop(state) do
    receive_loop(<<>>, state)
  end

  defp receive_loop(prev_rest, %{sock: sock}=state) do
    with {:ok, packet} <- :gen_tcp.recv(sock, 0, @recv_timeout) do
      rest = handle_packet(prev_rest <> packet, state)
      receive_loop(rest, state)
    end
  end

  defp handle_packet(packet, %{client_id: client_id, conn_pid: conn_pid}) do
    {messages, errors, rest} = Message.decode_all(packet)
    Enum.each(messages, &Client.send_conn_msg(client_id, &1))
    Enum.each(errors, &send_error_back(conn_pid, &1))
    rest
  end

  defp send_error_back(conn_pid, :invalid_message_type) do
    prob = {:prob, :bad_type}
    send(conn_pid, {:server_msg, prob})
  end

end
