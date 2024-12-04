defmodule Tccex.Client do
  use GenServer, restart: :temporary

  require Logger

  alias Tccex.{Message, Sumall}

  # XXX: could've just used the sock given by the tcp messages instead of storing it as the GenServer state

  def start_link(sock) do
    GenServer.start_link(__MODULE__, sock)
  end

  def init(sock) do
    {:ok, sock, {:continue, :turn_active}}
  end

  defp tcp_send(msg, sock) do
    packet = Message.encode(msg)
    with :ok <- :gen_tcp.send(sock, packet) do
      {:noreply, sock}
    else
      {:error, reason} ->
        Logger.warning("send failed: #{inspect(reason)}")
        {:stop, reason, sock}
    end
  end

  def handle_continue(:turn_active, sock) do
    :inet.setopts(sock, active: true)
    {:noreply, sock}
  end

  def handle_info({:tcp_closed, _sock}, sock) do
    {:stop, :normal, sock}
  end

  def handle_info({:tcp_error, _sock, reason}, sock) do
    {:stop, {:tcp_error, reason}, sock}
  end

  def handle_info({:tcp, _sock, data}, sock) do
    with {:ok, msg} <- Message.decode(data) do
      handle_msg(msg, sock)
    else
      {:error, error} ->
        {:stop, error, sock}
    end
  end

  def handle_info({:pub, topic, payload}, sock) do
    tcp_send({:pub, topic, payload}, sock)
  end


  defp handle_msg(:ping, sock) do
    tcp_send(:ping, sock)
  end

  defp handle_msg({:sub, topic}, sock) do
    Registry.register(Tccex.Topic.Registry, topic, nil)
    tcp_send({:sub, topic}, sock)
  end

  defp handle_msg({:unsub, topic}, sock) do
    Registry.unregister(Tccex.Topic.Registry, topic)
    tcp_send({:unsub, topic}, sock)
  end

  @prefix "!sumall "
  @prefixSize byte_size(@prefix)

  defp handle_msg({:pub, topic, payload}, sock) do
    if String.starts_with?(payload, @prefix) do
      # problem with this: the task keeps going even if the client terminates
      # should've done .async(), I guess?
      Task.Supervisor.start_child(Tccex.Task.Supervisor, fn ->
        payload =
          payload
          |> String.byte_slice(@prefixSize, byte_size(payload)-@prefixSize)
          |> Sumall.of
        dispatch(topic, payload)
      end)
    else
      dispatch(topic, payload)
    end
    {:noreply, sock}
  end

  defp dispatch(topic, payload) do
    Registry.dispatch(Tccex.Topic.Registry, topic, fn entries ->
      Enum.each(entries, fn {pid, _value} ->
        send(pid, {:pub, topic, payload})
      end)
    end)

  end
end
