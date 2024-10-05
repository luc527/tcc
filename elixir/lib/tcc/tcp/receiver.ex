defmodule Tcc.Tcp.Receiver do
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
    Enum.each(messages, &send_conn_msg(client_id, conn_pid, &1))
    Enum.each(errors, &send_error_back(conn_pid, &1, nil))
    rest
  end

  defp send_conn_msg(client_id, conn_pid, msg) do
    case Client.send_conn_msg(client_id, msg) do
      :ok -> :ok
      error ->
        mtype = if is_tuple(msg), do: elem(msg, 0), else: msg
        send_error_back(conn_pid, error, mtype)
    end
  end

  defp send_error_back(conn_pid, error, mtype) do
    send_prob_back(conn_pid, error |> error_to_prob(mtype))
  end

  defp send_prob_back(conn_pid, prob) do
    send(conn_pid, {:server_msg, prob})
  end

  defp error_to_prob(:invalid_message_type, _mtype) do
    {:prob, :bad_type}
  end

  defp error_to_prob({:error, error}, mtype) do
    if Message.is_message_error(error) do
      {:prob, error}
    else
      {:prob, {:transient, mtype}}
    end
  end

end
