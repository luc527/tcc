defmodule Tccex.Client do
  require Logger
  alias Tccex.{Message, Topic}
  use GenServer, restart: :transient

  def start_link(init_arg) do
    GenServer.start_link(__MODULE__, init_arg)
  end

  def init(sock) do
    {:ok, sock}
  end

  def handle_continue({:response, msg}, sock) do
    send_back(msg, sock)
  end

  def handle_cast({:pub, _topic, _payload} = msg, sock) do
    send_back(msg, sock)
  end

  defp send_back(msg, sock) do
    Logger.info("sending msg #{inspect(msg)}")
    packet = Message.encode(msg)

    with :ok <- :gen_tcp.send(sock, packet) do
      {:noreply, sock}
    else
      {:error, reason} ->
        Logger.warning("client: send failed with reason #{inspect(reason)}, stopping")
        {:stop, :normal, sock}
    end
  end

  def handle_call({:conn_msg, msg}, _from, sock) do
    handle_conn_msg(msg, sock)
  end

  defp handle_conn_msg(:ping, sock) do
    Logger.info("received ping, responding ping")
    {:reply, :ok, sock, {:continue, {:response, :ping}}}
  end

  defp handle_conn_msg({:sub, topic, b}, sock) do
    Logger.info("received sub #{topic} (#{inspect(b)})")
    handle_sub(topic, b)
    {:reply, :ok, sock, {:continue, {:response, {:sub, topic, b}}}}
  end

  defp handle_conn_msg({:pub, topic, payload}, sock) do
    Logger.info("received pub #{topic};#{payload}, publishing")
    handle_pub(topic, payload)
    {:reply, :ok, sock}
  end

  defp handle_sub(topic, true), do: Registry.register(Topic.Registry, topic, nil)
  defp handle_sub(topic, false), do: Registry.unregister(Topic.Registry, topic)

  defp handle_pub(topic, payload) do
    Registry.dispatch(Topic.Registry, topic, fn clients ->
      Enum.each(clients, fn {pid, _value} ->
        GenServer.cast(pid, {:pub, topic, payload})
      end)
    end)
  end

  def ping(pid), do: GenServer.call(pid, {:conn_msg, :ping})
  def sub(pid, topic, b), do: GenServer.call(pid, {:conn_msg, {:sub, topic, b}})
  def pub(pid, topic, payload), do: GenServer.call(pid, {:conn_msg, {:pub, topic, payload}})
end
