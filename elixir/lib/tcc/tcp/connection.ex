defmodule Tcc.Tcp.Connection do
  require Logger
  alias Tcc.{Message, Tcp}

  @ping_interval 20_000

  def start(sock) do
    Task.start(fn -> start_inner(sock) end)
  end

  defp start_inner(sock) do
    conn_pid = self()

    state = %{
      sock: sock,
      conn_pid: conn_pid,
      receiver_ref: nil,
      client_id: nil
    }

    dispatch_ping()
    loop(state)
  end

  defp dispatch_ping() do
    Process.send_after(self(), :send_ping, @ping_interval)
  end

  defp loop(state) do
    case step(state) do
      {:cont, state} ->
        loop(state)

      {:stop, _reason} ->
        :ok
    end
  end

  defp step(%{receiver_ref: receiver_ref} = state) do
    receive do
      :send_ping ->
        send_to_socket_then(:ping, state, &dispatch_ping/0)

      {:server_msg, msg} ->
        send_to_socket(msg, state)

      {:start_receiver, client_id} ->
        state = %{state | client_id: client_id}
        %{ref: ref} = Task.async(fn -> Tcp.Receiver.receive_loop(state) end)
        state = %{state | receiver_ref: ref}
        {:cont, state}

      {^receiver_ref, reply} ->
        demonitor(receiver_ref)
        {:stop, {:receiver_closed, reply}}

      {:DOWN, ^receiver_ref, _, _, reason} ->
        {:stop, {:receiver_exited, reason}}

      msg ->
        Logger.error("connection: invalid message: #{inspect(msg)}")
        {:cont, state}
    end
  end

  defp send_to_socket(msg, state) do
    send_to_socket_then(msg, state, nil)
  end

  defp send_to_socket_then(msg, %{sock: sock} = state, after_fn) do
    packet = Message.encode(msg)

    with :ok <- :gen_tcp.send(sock, packet) do
      after_fn && after_fn.()
      {:cont, state}
    else
      error ->
        handle_tcp_send_error(error, state)
    end
  end

  defp handle_tcp_send_error({:error, :timeout}, state), do: shutdown(state)
  defp handle_tcp_send_error({:error, reason}, _state), do: {:stop, reason}
  defp handle_tcp_send_error(error, _state), do: {:stop, error}

  defp shutdown(%{sock: sock, receiver_ref: receiver_ref}) do
    with :ok <- :gen_tcp.shutdown(sock, :write) do
      {:stop, :shutdown}
    else
      {:error, reason} -> {:stop, reason}
      error -> {:stop, error}
    end

    demonitor(receiver_ref)
  end

  defp demonitor(nil), do: false

  defp demonitor(ref) do
    Process.demonitor(ref, [:flush])
  end

  def start_receiver(conn_pid, client_id) do
    send(conn_pid, {:start_receiver, client_id})
  end
end
