defmodule Tccex.Tcp.Connection do
  require Logger
  alias Tccex.Message
  alias Tccex.Client

  @send_timeout 10_000
  @ping_interval 20_000
  @recv_timeout 30_000

  def start(sock) do
    Logger.info("connection: starting connection, sock #{inspect(sock)}")

    opts = [
      send_timeout: @send_timeout,
      send_timeout_close: true
    ]

    with :ok <- :inet.setopts(sock, opts) do
      Task.start(fn -> run(sock) end)
    end
  end

  def run(sock) do
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
    Logger.info("connection: dispatching ping")
    Process.send_after(self(), :send_ping, @ping_interval)
  end

  defp loop(state) do
    case step(state) do
      {:cont, state} ->
        loop(state)

      {:stop, reason} ->
        Logger.info("connection: stopped with reason #{inspect(reason)}")
        :ok
    end
  end

  defp step(%{receiver_ref: receiver_ref}=state) do
    receive do
      :send_ping ->
        send_to_socket_then(:ping, state, &dispatch_ping/0)

      {:send, message} ->
        send_to_socket(message, state)

      # TODO: all this stuff looks a little awkward

      {:start_receiver, client_id} ->
        Logger.info("connection: starting receiver")
        state = %{state | client_id: client_id}
        %{ref: ref} = Task.async(fn -> run_receiver(state) end)
        state = %{state | receiver_ref: ref}
        {:cont, state}

      {^receiver_ref, reply} ->
        Logger.info("connection: receiver closed")
        demonitor(receiver_ref)
        {:stop, {:receiver_closed, reply}}

      {:DOWN, ^receiver_ref, _, _, reason} ->
        Logger.info("connection: receiver terminated! #{inspect(reason)}")
        {:stop, {:receiver_exited, reason}}

      msg ->
        Logger.error("connection: invalid message #{inspect(msg)}")
    end
  end

  defp send_to_socket(message, state) do
    send_to_socket_then(message, state)
  end

  defp send_to_socket_then(message, %{sock: sock} = state, after_fn \\ nil) do
    packet = Message.encode(message)

    with :ok <- :gen_tcp.send(sock, packet) do
      after_fn && after_fn.()
      {:cont, state}
    else
      error ->
        handle_send_error(error, state)
    end
  end

  defp handle_send_error({:error, :timeout}, state), do: shutdown(state)
  defp handle_send_error({:error, reason}, _state), do: {:stop, reason}
  defp handle_send_error(error, _state), do: {:stop, error}

  defp shutdown(%{sock: sock, receiver_ref: receiver_ref}) do
    with :ok <- :gen_tcp.shutdown(sock, :write) do
      {:stop, :shutdown}
    else
      {:error, reason} ->
        {:stop, reason}

      error ->
        {:stop, error}
    end

    demonitor(receiver_ref)
  end

  defp demonitor(nil), do: false

  defp demonitor(ref) do
    Process.demonitor(ref, [:flush])
  end

  defp run_receiver(state) do
    recv_loop(<<>>, state)
  end

  defp recv_loop(prev_rest, %{sock: sock} = state) do
    with {:ok, packet} <- :gen_tcp.recv(sock, 0, @recv_timeout) do
      rest = handle_packet(prev_rest <> packet, state)
      recv_loop(rest, state)
    else
      {:error, :timeout} ->
        :ok

      {:error, :closed} ->
        :ok
    end
  end

  defp handle_packet(packet, %{client_id: client_id, conn_pid: conn_pid}) do
    {messages, errors, rest} = Message.decode_all(packet)
    Enum.each(messages, &recv_message(client_id, &1))
    Enum.each(errors, &send_decode_error(conn_pid, &1))
    rest
  end

  defp recv_message(client_id, message) do
    Client.recv(client_id, message)
  end

  defp decode_error_to_prob(:invalid_message_type), do: {:prob, :bad_type}

  defp send_decode_error(conn_pid, error) do
    send(conn_pid, {:send, error |> decode_error_to_prob()})
  end

  def start_receiver(conn_pid, client_id) do
    send(conn_pid, {:start_receiver, client_id})
  end
end
